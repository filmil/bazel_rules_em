# SPDX-License-Identifier: Apache-2.0
"""Tests of sim.py's parts that need no solver."""

import os
import tempfile
import unittest

import numpy as np

import geometry
import sim


def model(layers=2):
    """A small model: a 10 x 6 board, a track on F.Cu, a pour on the
    bottom layer, a slot, two ports."""
    names = ["F.Cu"] + ["In%d.Cu" % k for k in range(1, layers - 1)] + ["B.Cu"]
    copper, dielectrics, z = [], [], 0.0
    for k, name in enumerate(reversed(names)):
        copper.insert(0, {"name": name, "z": z, "thickness": 0.035})
        z += 0.035
        if k < layers - 1:
            dielectrics.insert(0, {"name": "dielectric %d" % (layers - 1 - k), "z": z, "thickness": 0.5,
                                   "epsilon_r": 4.5, "loss_tangent": 0.02})
            z += 0.5
    return {
        "board": {"min_x": 0, "min_y": 0, "max_x": 10, "max_y": 6, "thickness": z},
        "copper": copper,
        "dielectrics": dielectrics,
        "nets": ["S", "GND"],
        "ports": [
            {"name": "P1", "pad": "J1.1", "net": "S", "layer": "F.Cu", "x": 1, "y": 3},
            {"name": "P2", "pad": "J2.1", "net": "S", "layer": "F.Cu", "x": 9, "y": 3},
        ],
        "holes": [{"net": "GND", "x": 5, "y": 1, "drill": 0.3, "size": 0.6, "layers": names,
                   "slot": [[4.8, 0.9], [5.2, 0.9], [5.2, 1.1], [4.8, 1.1]]}],
        "shapes": [
            {"layer": "F.Cu", "net": "S", "points": [[1, 2.95], [9, 2.95], [9, 3.05], [1, 3.05]]},
            {"layer": "B.Cu", "net": "GND", "points": [[0, 0], [10, 0], [10, 6], [0, 6]]},
        ],
    }


class Recorder:
    """Stands in for a CSXCAD structure or property: records each call."""

    def __init__(self, log):
        self.log = log

    def __getattr__(self, name):
        def call(*args, **kw):
            self.log.append((name, args, kw))
            return Recorder(self.log)

        return call


class HzTest(unittest.TestCase):
    def test_units(self):
        self.assertEqual(sim.hz("6GHz"), 6e9)
        self.assertEqual(sim.hz("100 MHz"), 1e8)
        self.assertEqual(sim.hz("2.4e9Hz"), 2.4e9)
        self.assertEqual(sim.hz("50kHz"), 5e4)

    def test_bad(self):
        for bad in ["6", "6 GHZ", "-1GHz", "0Hz", "nanGHz", "infHz", "GHz"]:
            with self.assertRaises(ValueError, msg=bad):
                sim.hz(bad)


class GeometryTest(unittest.TestCase):
    def test_sheets(self):
        m = model(4)
        self.assertEqual(geometry.sheet_z(m, "F.Cu"), m["copper"][0]["z"])
        self.assertEqual(geometry.sheet_z(m, "B.Cu"), 0.035)
        inner = m["copper"][1]
        self.assertAlmostEqual(geometry.sheet_z(m, "In1.Cu"), inner["z"] + 0.0175)

    def test_kappa_and_sheets(self):
        log = []
        geometry.build(Recorder(log), model(), sheets=True, frequency=1e9)
        materials = [kw for name, _, kw in log if name == "AddMaterial"]
        self.assertAlmostEqual(materials[0]["kappa"], 2 * np.pi * 1e9 * geometry.EPS0 * 4.5 * 0.02)
        names = [name for name, _, _ in log]
        # Copper as sheets; only the slot, through the board, has height.
        self.assertEqual(names.count("AddPolygon"), 2)
        self.assertEqual(names.count("AddLinPoly"), 1)

    def test_inner_slab(self):
        log = []
        geometry.build(Recorder(log), model(4))
        boxes = [args for name, args, _ in log if name == "AddBox"]
        # Three dielectrics, and the two inner copper slabs filled.
        self.assertEqual(len(boxes), 5)


class PortTest(unittest.TestCase):
    def test_reference_below(self):
        start, stop, note = sim.port_box(model(), model()["ports"][0])
        self.assertIsNone(note)
        self.assertEqual(start[2], 0.035)
        self.assertEqual(stop[2], model()["copper"][0]["z"])

    def test_skips_empty_layers(self):
        # On four layers with the pour on B.Cu, the port reaches past the
        # empty inner layers to B.Cu.
        m = model(4)
        layer, note = sim.port_reference(m, m["ports"][0])
        self.assertEqual((layer, note), ("B.Cu", None))

    def test_no_reference(self):
        m = model()
        m["shapes"] = m["shapes"][:1]
        layer, note = sim.port_reference(m, m["ports"][0])
        self.assertEqual(layer, "B.Cu")
        self.assertIn("no modelled copper under P1", note)

    def test_one_layer(self):
        m = model()
        m["copper"] = m["copper"][:1]
        with self.assertRaisesRegex(ValueError, "one copper layer"):
            sim.port_reference(m, m["ports"][0])


class MeshTest(unittest.TestCase):
    def test_lines(self):
        from CSXCAD import ContinuousStructure

        m = model()
        csx = ContinuousStructure()
        boxes = [sim.port_box(m, p)[:2] for p in m["ports"]]
        cells = sim.mesh(csx, m, {"mesh_mm": 0.25, "air_mm": 2}, boxes)
        grid = csx.GetGrid()
        x = list(grid.GetLines("x"))
        y = list(grid.GetLines("y"))
        z = list(grid.GetLines("z"))
        self.assertEqual(cells, (len(x) - 1) * (len(y) - 1) * (len(z) - 1))
        # The 0.1 mm track's centre, the ports' sides and the hole's centre
        # are lines, whatever lies near them.
        for v in [3.0, 2.9, 3.1, 1.0]:
            self.assertTrue(any(abs(v - l) < 1e-6 for l in y), v)
        for v in [0.9, 1.1, 8.9, 9.1, 5.0]:
            self.assertTrue(any(abs(v - l) < 1e-6 for l in x), v)
        # No cell over 0.25 mm, and the dielectric in four or more.
        for lines in (x, y, z):
            self.assertLessEqual(max(np.diff(lines)), 0.25 + 1e-9)
        d = m["dielectrics"][0]
        self.assertGreaterEqual(sum(1 for l in z if d["z"] - 1e-9 <= l <= d["z"] + d["thickness"] + 1e-9), 5)

    def test_merge_tiers(self):
        # The first tier whole (but 1 nm duplicates); 1.02 too close to it;
        # the centres 1.5 and 1.51 thinned; the edge 1.55 too close.
        self.assertEqual(sim.merge([[1.0, 2.0, 1.0000001], [1.02, 1.5, 1.51], [1.55, 1.99]], 0.1),
                         [1.0, 1.5, 2.0])

    def test_no_thin_cells(self):
        # An arc's many short segments: their centres microns apart.
        from CSXCAD import ContinuousStructure

        m = model()
        m["shapes"] += [{"layer": "F.Cu", "net": "S",
                         "points": [[4 + k * 0.001, 2.9], [4.1 + k * 0.001, 2.9], [4.1 + k * 0.001, 3.1],
                                    [4 + k * 0.001, 3.1]]} for k in range(20)]
        csx = ContinuousStructure()
        sim.mesh(csx, m, {"mesh_mm": 0.25, "air_mm": 2}, [sim.port_box(m, p)[:2] for p in m["ports"]])
        x = list(csx.GetGrid().GetLines("x"))
        self.assertGreater(min(np.diff(x)), 0.25 / 4 - 1e-6)


class NumbersTest(unittest.TestCase):
    def test_bad(self):
        for key, bad in [("mesh_mm", "0"), ("mesh_mm", "nan"), ("air_mm", "-1"), ("z0", "inf"), ("z0", "x")]:
            with self.assertRaises(ValueError, msg=(key, bad)):
                sim.positive({key: bad}, key)

    def test_band(self):
        params = {"f_start": "2GHz", "f_stop": "1GHz", "points": 2, "z0": 50, "air_mm": 2, "mesh_mm": 0.25}
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "m.json")
            with open(p, "w") as f:
                f.write('{"ports": [{"name": "P1"}]}')
            params["model"] = p
            with self.assertRaisesRegex(ValueError, "not below"):
                sim.simulate(d, params, None)


class AirTest(unittest.TestCase):
    def test_too_little_air(self):
        params = {"model": "/nonexistent", "f_start": "1GHz", "f_stop": "2GHz", "points": 2,
                  "z0": 50, "air_mm": 1, "mesh_mm": 0.25}
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "m.json")
            with open(p, "w") as f:
                f.write('{"ports": [{"name": "P1"}]}')
            params["model"] = p
            with self.assertRaisesRegex(ValueError, "8 cells"):
                sim.simulate(d, params, None)


class MergeTest(unittest.TestCase):
    def test_merge(self):
        self.assertEqual(sim.merge([[], [3, 1, 1.05, 2, 1.2]], 0.1), [1, 1.2, 2, 3])


class TouchstoneTest(unittest.TestCase):
    def write(self, n):
        S = np.zeros((n, n, 2), dtype=complex)
        for i in range(n):
            for j in range(n):
                S[i, j] = complex(i + 1, j + 1)
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "t")
            sim.touchstone(p, np.array([1e9, 2e9]), S, 50)
            with open(p) as f:
                return f.read().splitlines()

    def test_two_ports(self):
        lines = self.write(2)
        self.assertEqual(lines[1], "# Hz S RI R 50")
        # S11 S21 S12 S22.
        f = lines[2].split()
        self.assertEqual(float(f[0]), 1e9)
        vals = [float(x) for x in f[1:]]
        self.assertEqual(vals, [1, 1, 2, 1, 1, 2, 2, 2])

    def test_one_and_three_ports(self):
        lines = self.write(1)
        self.assertEqual([float(x) for x in lines[2].split()], [1e9, 1, 1])
        lines = self.write(3)
        # One line per row, the first starting with the frequency.
        self.assertEqual(len(lines), 2 + 2 * 3)
        self.assertEqual(len(lines[2].split()), 7)
        self.assertEqual(len(lines[3].split()), 6)

    def test_five_ports(self):
        lines = self.write(5)
        # Each row of five pairs takes two lines: four pairs, then one.
        self.assertEqual(len(lines), 2 + 2 * 5 * 2)
        self.assertEqual(len(lines[2].split()), 9)
        self.assertEqual(len(lines[3].split()), 2)

    def test_four_ports(self):
        lines = self.write(4)
        # One frequency: four rows of four pairs.
        rows = lines[2:6]
        self.assertEqual(float(rows[0].split()[0]), 1e9)
        self.assertEqual(len(rows[0].split()), 9)
        self.assertEqual([float(x) for x in rows[1].split()], [2, 1, 2, 2, 2, 3, 2, 4])


class TdrTest(unittest.TestCase):
    def test_matched(self):
        _, z = sim.tdr(np.zeros(64, dtype=complex), 1e8, 50)
        np.testing.assert_allclose(z, 50)

    def test_open_and_short(self):
        # A pure reflection settles at the open's clip and the short's.
        t, z = sim.tdr(np.ones(64, dtype=complex), 1e8, 50)
        self.assertGreater(z[np.argmin(abs(t - 1e-9))], 1000)
        t, z = sim.tdr(-np.ones(64, dtype=complex), 1e8, 50)
        self.assertLess(z[np.argmin(abs(t - 1e-9))], 1)

    def test_delay(self):
        # A 100 ohm load 200 ps away (round trip): 50 ohm before, 100 after.
        n, df = 512, 6e9 / 511
        f = np.arange(n) * df
        s11 = (1 / 3) * np.exp(-2j * np.pi * f * 200e-12)
        t, z = sim.tdr(s11, df, 50)
        self.assertLess(t[0], 0)
        self.assertAlmostEqual(z[np.argmin(abs(t - 0))], 50, delta=1)
        self.assertAlmostEqual(z[np.argmin(abs(t - 600e-12))], 100, delta=2)
        # Half-way up near 200 ps.
        r = (z - 50) / (z + 50)
        half = t[np.argmax(r >= 1 / 6)]
        self.assertAlmostEqual(half, 200e-12, delta=20e-12)

    def test_rise_time(self):
        # About 1 / f_stop for the Hann-windowed step.
        rise = sim.rise_time(512, 6e9 / 511)
        self.assertGreater(rise, 0.5 / 6e9)
        self.assertLess(rise, 1.5 / 6e9)

    def test_load(self):
        # A 100 ohm load on 50 ohm: S11 = 1/3.
        t, z = sim.tdr(np.full(256, 1 / 3, dtype=complex), 1e8, 50)
        self.assertAlmostEqual(z[np.argmin(abs(t - 1e-9))], 100, delta=2)


class FontsTest(unittest.TestCase):
    def test_own_fonts_only(self):
        import matplotlib

        plt = sim._pyplot()
        from matplotlib import font_manager

        self.assertTrue(font_manager.fontManager.ttflist)
        for f in font_manager.fontManager.ttflist:
            self.assertIn(sim.OWN_FONTS, f.fname)
        found = font_manager.findfont(font_manager.FontProperties(family=["sans-serif"]))
        self.assertIn(sim.OWN_FONTS, found)
        self.assertEqual(os.environ["PATH"], "")
        self.assertIsNotNone(matplotlib)
        self.assertIsNotNone(plt)


if __name__ == "__main__":
    unittest.main()
