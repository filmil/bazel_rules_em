# SPDX-License-Identifier: Apache-2.0
"""kicad_em_sim's solver runs: a kicad_em_model's ports, each excited in
turn, and what follows from them.

The openEMS runner runs it (simulate(out_dir, params, run)). params:

  model           the model.json path
  f_start, f_stop the band, such as "100MHz" and "6GHz"
  points          the number of frequencies in it
  mesh_mm         the largest mesh cell, mm
  air_mm          the air around the board, mm; the absorbing boundary (8
                  cells of PML) lies in it, so at least 8 mesh_mm
  z0              the ports' reference impedance, ohm
  steps           the time steps of each run, exactly
  name            the base name of the outputs

The copper is drawn as sheets (geometry.build(sheets=True)). Each port is
a lumped port of z0 from its pad's sheet to the next copper layer's sheet
(the layer below it, or above it for the bottom layer), as a probe would
measure the pad against the copper under it. Every port is excited once,
so the whole S-matrix is known. It writes into out_dir:

  <name>.s<N>p      Touchstone 1.1: Hz, S, real and imaginary, R z0
  s_params.png      |S| in dB of every pair
  impedance.png     each port's input impedance, real and imaginary
  tdr.png           each port's TDR impedance, from its S11 (a step,
                    rise time 0.35 / f_stop)
  report.md         the setup, the mesh, and the numbers behind the plots
"""

import json
import math
import os
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import geometry  # noqa: E402

# The pad's side of a lumped port, mm.
PORT_HALF = 0.1

_UNITS = {"Hz": 1.0, "kHz": 1e3, "MHz": 1e6, "GHz": 1e9}


def hz(s):
    """A frequency: "6GHz", "100 MHz", "2.4e9Hz"; finite and above 0."""
    s = str(s).strip()
    for unit in sorted(_UNITS, key=len, reverse=True):
        if s.endswith(unit):
            try:
                v = float(s[: -len(unit)].strip()) * _UNITS[unit]
            except ValueError:
                break
            if v > 0 and math.isfinite(v):
                return v
            break
    raise ValueError("frequency %r: want a number and Hz, kHz, MHz or GHz, such as 6GHz" % s)


def port_box(m, p):
    """A port's start and stop corners: from its pad's sheet to the next
    copper layer's sheet."""
    names = [c["name"] for c in m["copper"]]
    i = names.index(p["layer"])
    other = names[i + 1] if i + 1 < len(names) else names[i - 1]
    z1, z2 = geometry.sheet_z(m, p["layer"]), geometry.sheet_z(m, other)
    x, y = p["x"], p["y"]
    # The port runs from the pad down (or up) to the reference copper; its
    # sign makes the current flow into the pad.
    return [x - PORT_HALF, y - PORT_HALF, z2], [x + PORT_HALF, y + PORT_HALF, z1]


def merge(values, gap):
    """values, sorted, without any closer than gap to the one kept before
    it (very small cells would make the time step small)."""
    out = []
    for v in sorted(values):
        if not out or v - out[-1] >= gap:
            out.append(v)
    return out


def mesh(csx, m, params, boxes):
    """A rectilinear mesh: the board's box with air around it; lines at
    the ports, at every sheet, and at each copper shape's edges and centre
    (a track between two lines would be dropped); merged where closer
    than a quarter cell, then smoothed to at most mesh_mm."""
    from CSXCAD.SmoothMeshLines import SmoothMeshLines

    b = m["board"]
    air = float(params["air_mm"])
    res = float(params["mesh_mm"])
    lines = {
        "x": [b["min_x"] - air, b["min_x"], b["max_x"], b["max_x"] + air],
        "y": [b["min_y"] - air, b["min_y"], b["max_y"], b["max_y"] + air],
        "z": [-air, m["copper"][0]["z"] + m["copper"][0]["thickness"] + air],
    }
    for c in m["copper"]:
        lines["z"].append(geometry.sheet_z(m, c["name"]))
    for start, stop in boxes:
        lines["x"] += [start[0], stop[0]]
        lines["y"] += [start[1], stop[1]]
    for sh in m["shapes"]:
        xs = [p[0] for p in sh["points"]]
        ys = [p[1] for p in sh["points"]]
        lines["x"] += [min(xs), max(xs), (min(xs) + max(xs)) / 2]
        lines["y"] += [min(ys), max(ys), (min(ys) + max(ys)) / 2]
    for h in m["holes"]:
        lines["x"].append(h["x"])
        lines["y"].append(h["y"])
    grid = csx.GetGrid()
    grid.SetDeltaUnit(1e-3)
    cells = 1
    for axis in "xyz":
        # Dielectrics get at least four cells across.
        r = res
        if axis == "z":
            r = min([res] + [d["thickness"] / 4 for d in m["dielectrics"]])
        fixed = merge([round(v, 6) for v in lines[axis]], r / 4)
        pts = SmoothMeshLines(fixed, r, 1.4)
        grid.SetLines(axis, pts)
        cells *= len(pts) - 1
    return cells


def touchstone(path, freqs, S, z0):
    """Writes a Touchstone 1.1 file: Hz, S, RI."""
    n = S.shape[0]
    with open(path, "w") as f:
        f.write("! kicad_em_sim: S-parameters of %d ports, openEMS\n" % n)
        f.write("# Hz S RI R %g\n" % z0)
        for k, fr in enumerate(freqs):
            if n == 2:
                # Touchstone 1's two-port order: S11 S21 S12 S22.
                order = [(0, 0), (1, 0), (0, 1), (1, 1)]
            else:
                order = [(i, j) for i in range(n) for j in range(n)]
            vals = ["%.6e %.6e" % (S[i, j, k].real, S[i, j, k].imag) for i, j in order]
            if n <= 2:
                f.write("%.6e %s\n" % (fr, " ".join(vals)))
                continue
            # Larger matrices: a row of S per line group, four pairs a line.
            for i in range(n):
                row = vals[i * n:(i + 1) * n]
                for c in range(0, n, 4):
                    lead = "%.6e" % fr if i == 0 and c == 0 else " " * 12
                    f.write("%s %s\n" % (lead, " ".join(row[c:c + 4])))


def tdr(s11, df, z0):
    """The TDR impedance from S11 on 0, df, 2 df, ...: the reflection of a
    step (a Hann window sets its rise time), as Z0 (1 + r) / (1 - r).
    Returns (time in s, impedance in ohm)."""
    import numpy as np

    n = len(s11)
    m = 2 * (n - 1)
    window = np.hanning(2 * n)[n:]
    impulse = np.fft.irfft(s11 * window, m)
    # The impulse is spread both sides of t = 0, its early half wrapped to
    # the array's end: put t = 0 in the middle, integrate from there back.
    step = np.cumsum(np.roll(impulse, m // 2))[m // 2:]
    step = np.clip(step, -0.999, 0.999)
    dt = 1.0 / (m * df)
    return np.arange(len(step)) * dt, z0 * (1 + step) / (1 - step)


def plot(path, title, xlabel, ylabel, series):
    """A line plot, written without a time or version in its metadata."""
    import matplotlib

    matplotlib.use("Agg")
    from matplotlib import pyplot as plt

    fig, ax = plt.subplots(figsize=(8, 5), dpi=100)
    for label, x, y in series:
        ax.plot(x, y, label=label, linewidth=1.2)
    ax.set_title(title)
    ax.set_xlabel(xlabel)
    ax.set_ylabel(ylabel)
    ax.grid(True, alpha=0.4)
    if len(series) > 1:
        ax.legend(fontsize=8)
    fig.tight_layout()
    fig.savefig(path, metadata={"Software": None})
    plt.close(fig)


def simulate(out_dir, params, run):
    import numpy as np

    with open(params["model"]) as f:
        m = json.load(f)
    ports = m["ports"]
    n = len(ports)
    if n == 0:
        raise ValueError("the model has no ports: give kicad_em_model's ports")
    f_start, f_stop = hz(params["f_start"]), hz(params["f_stop"])
    if f_start >= f_stop:
        raise ValueError("f_start %s is not below f_stop %s" % (params["f_start"], params["f_stop"]))
    points = int(params["points"])
    z0 = float(params["z0"])
    if float(params["air_mm"]) < 8 * float(params["mesh_mm"]):
        raise ValueError("air_mm %s is less than 8 cells of mesh_mm %s: the absorbing boundary (PML) "
                         "takes 8 cells" % (params["air_mm"], params["mesh_mm"]))
    freqs = np.linspace(f_start, f_stop, points)
    # The TDR's own grid, from DC: 0, df, ..., f_stop.
    tdr_n = 512
    tdr_df = f_stop / (tdr_n - 1)
    tdr_freqs = np.arange(1, tdr_n) * tdr_df
    S = np.zeros((n, n, points), dtype=complex)
    S11_tdr = {}
    zin = {}
    cells = 0
    from CSXCAD import ContinuousStructure
    from openEMS import openEMS

    for i, excited in enumerate(ports):
        csx = ContinuousStructure()
        geometry.build(csx, m, sheets=True, frequency=(f_start + f_stop) / 2)
        # Exactly `steps` time steps: openEMS checks its end criterion at
        # wall-clock intervals, so a run that stopped on it would take a
        # different number of steps, and give other numbers, on a faster
        # or busier machine. The criterion is set out of reach.
        fdtd = openEMS(NrTS=int(params["steps"]), EndCriteria=1e-30)
        # A Gaussian pulse from DC to f_stop.
        fdtd.SetGaussExcite(f_stop / 2, f_stop / 2)
        # PML of 8 cells: the energy leaves the board within the run's
        # steps, so its signals are not cut short (Mur's boundary reflects
        # enough that they ring on).
        fdtd.SetBoundaryCond(["PML_8"] * 6)
        fdtd.SetCSX(csx)
        boxes = [port_box(m, p) for p in ports]
        lumped = [
            fdtd.AddLumpedPort(j + 1, z0, start, stop, "z", excite=1.0 if j == i else 0, priority=20)
            for j, (start, stop) in enumerate(boxes)
        ]
        cells = mesh(csx, m, params, boxes)
        with tempfile.TemporaryDirectory() as sim_path:
            run(fdtd, sim_path)
            for p in lumped:
                p.CalcPort(sim_path, freqs, ref_impedance=z0)
            for j, p in enumerate(lumped):
                S[j, i] = p.uf_ref / lumped[i].uf_inc
            zin[excited["name"]] = lumped[i].uf_tot / lumped[i].if_tot
            lumped[i].CalcPort(sim_path, tdr_freqs, ref_impedance=z0)
            s11 = lumped[i].uf_ref / lumped[i].uf_inc
            # S11 at DC: the lowest frequency's, made real.
            S11_tdr[excited["name"]] = np.concatenate([[s11[0].real], s11])

    name = params["name"]
    names = [p["name"] for p in ports]
    touchstone(os.path.join(out_dir, "%s.s%dp" % (name, n)), freqs, S, z0)
    ghz = freqs / 1e9
    db = lambda s: 20 * np.log10(np.maximum(np.abs(s), 1e-12))  # noqa: E731
    plot(os.path.join(out_dir, "s_params.png"), "S-parameters", "frequency (GHz)", "|S| (dB)",
         [("S%s,%s" % (names[a], names[b]), ghz, db(S[a, b])) for b in range(n) for a in range(n)])
    series = []
    for p in names:
        series += [("Re Z %s" % p, ghz, zin[p].real), ("Im Z %s" % p, ghz, zin[p].imag)]
    plot(os.path.join(out_dir, "impedance.png"), "Input impedance", "frequency (GHz)", "Z (ohm)", series)
    tdr_series = []
    tdr_ranges = {}
    for p in names:
        t, z = tdr(S11_tdr[p], tdr_df, z0)
        # The first half of the window: the board, not the wrap-around.
        keep = t < t[-1] / 2
        tdr_series.append(("Z %s" % p, t[keep] * 1e12, z[keep]))
        tdr_ranges[p] = (float(z[keep].min()), float(z[keep].max()))
    plot(os.path.join(out_dir, "tdr.png"), "TDR impedance (rise time %.0f ps)" % (0.35 / f_stop * 1e12),
         "time (ps)", "Z (ohm)", tdr_series)

    with open(os.path.join(out_dir, "report.md"), "w") as f:
        w = f.write
        w("# %s\n\n" % name)
        w("openEMS FDTD, %d ports, each excited in turn; copper as sheets.\n\n" % n)
        w("* Band: %.4g to %.4g GHz, %d points.\n" % (f_start / 1e9, f_stop / 1e9, points))
        w("* Mesh: %d cells, at most %g mm; %g mm of air, its outer 8 cells an absorbing boundary (PML).\n"
          % (cells, float(params["mesh_mm"]), float(params["air_mm"])))
        w("* Each run takes %d time steps.\n" % int(params["steps"]))
        w("* Ports: lumped, %g ohm, from the pad to the next copper layer.\n\n" % z0)
        w("| Port | Pad | Net | Layer |\n|---|---|---|---|\n")
        for p in ports:
            w("| %s | %s | %s | %s |\n" % (p["name"], p["pad"], p["net"], p["layer"]))
        w("\n## S-parameters (dB)\n\n")
        picks = sorted(set([0, points // 2, points - 1]))
        w("| | " + " | ".join("%.4g GHz" % ghz[k] for k in picks) + " |\n")
        w("|---|" + "---:|" * len(picks) + "\n")
        for b in range(n):
            for a in range(n):
                w("| S%s,%s | " % (names[a], names[b]) + " | ".join("%.2f" % db(S[a, b])[k] for k in picks) + " |\n")
        w("\n## TDR impedance\n\n| Port | min (ohm) | max (ohm) |\n|---|---:|---:|\n")
        for p in names:
            w("| %s | %.1f | %.1f |\n" % (p, tdr_ranges[p][0], tdr_ranges[p][1]))
        w("\n![S-parameters](s_params.png)\n\n![Input impedance](impedance.png)\n\n![TDR](tdr.png)\n")
