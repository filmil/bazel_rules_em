# SPDX-License-Identifier: Apache-2.0
"""Tests of sim.py's parts that need no solver."""

import os
import tempfile
import unittest

import numpy as np

import sim


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
        self.assertEqual(sim.merge([3, 1, 1.05, 2, 1.2], 0.1), [1, 1.2, 2, 3])


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
        _, z = sim.tdr(np.ones(64, dtype=complex), 1e8, 50)
        self.assertGreater(z[len(z) // 8], 1000)
        _, z = sim.tdr(-np.ones(64, dtype=complex), 1e8, 50)
        self.assertLess(z[len(z) // 8], 1)

    def test_load(self):
        # A 100 ohm load on 50 ohm: S11 = 1/3.
        _, z = sim.tdr(np.full(256, 1 / 3, dtype=complex), 1e8, 50)
        self.assertAlmostEqual(z[len(z) // 8], 100, delta=2)


if __name__ == "__main__":
    unittest.main()
