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
a lumped port of z0 from its pad's sheet to the sheet of the nearest
copper layer, going away from the pad's layer, with modelled copper under
the pad (the adjacent layer, noted in the report, if none has): as a probe
would measure the pad against the copper under it. Every port is excited once,
so the whole S-matrix is known. It writes into out_dir:

  <name>.s<N>p      Touchstone 1.1: Hz, S, real and imaginary, R z0
  s_params.png      |S| in dB of every pair
  impedance.png     each port's input impedance, real and imaginary
  tdr.png           each port's TDR impedance, from its S11: a step whose
                    rise time (about 1 / f_stop) the report gives
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


def inside(pt, poly):
    """Whether pt lies inside polygon poly (even-odd)."""
    x, y = pt
    hit = False
    for k in range(len(poly)):
        (x1, y1), (x2, y2) = poly[k], poly[k - 1]
        if (y1 > y) != (y2 > y) and x < x1 + (y - y1) * (x2 - x1) / (y2 - y1):
            hit = not hit
    return hit


def port_reference(m, p):
    """The copper layer a port's other end lands on: the nearest layer,
    going away from its pad's layer (down, or up from the bottom layer),
    with modelled copper under the pad. If none has, the adjacent layer,
    and a note saying so."""
    names = [c["name"] for c in m["copper"]]
    if len(names) < 2:
        raise ValueError("port %s: a model of one copper layer has nothing to reference" % p["name"])
    i = names.index(p["layer"])
    order = names[i + 1:] if i + 1 < len(names) else list(reversed(names[:i]))
    for layer in order:
        for sh in m["shapes"]:
            if sh["layer"] == layer and inside((p["x"], p["y"]), sh["points"]):
                return layer, None
    return order[0], ("no modelled copper under %s's pad on %s: its reference is that layer's "
                      "plane with nothing on it" % (p["name"], ", ".join(order)))


def port_box(m, p):
    """A port's start and stop corners, from its pad's sheet to its
    reference layer's sheet, and a note if the reference has no copper."""
    ref, note = port_reference(m, p)
    z1, z2 = geometry.sheet_z(m, p["layer"]), geometry.sheet_z(m, ref)
    x, y = p["x"], p["y"]
    return [x - PORT_HALF, y - PORT_HALF, z2], [x + PORT_HALF, y + PORT_HALF, z1], note


def positive(params, key):
    """params[key] as a finite number above 0."""
    try:
        v = float(params[key])
    except (TypeError, ValueError):
        v = float("nan")
    if not (v > 0 and math.isfinite(v)):
        raise ValueError("%s %r: want a finite number above 0" % (key, params[key]))
    return v


def merge(tiers, gap):
    """Mesh lines from tiers of candidates, the first tier first. The
    first tier is kept whole (only duplicates within 1 nm go); a line of a
    later tier is kept only at least gap from every line kept before it,
    so no cell is much thinner than gap (a thin cell makes the time step
    small)."""
    kept = []
    for v in sorted(tiers[0]):
        if not kept or v - kept[-1] > 1e-6:
            kept.append(v)
    for tier in tiers[1:]:
        for v in sorted(tier):
            if all(abs(v - k) >= gap for k in kept):
                kept.append(v)
                kept.sort()
    return kept


def mesh(csx, m, params, boxes):
    """A rectilinear mesh: the board's box with air around it, smoothed
    to at most mesh_mm, from lines in three tiers. First, kept whole: the
    box and air, each port's sides, each sheet and each dielectric's
    quarters. Then each copper shape's centre and each hole's centre, at
    least a quarter cell from the lines kept, so a track wider than half a
    cell keeps a line inside it. Then the shapes' and slots' edges, as
    room allows."""
    from CSXCAD.SmoothMeshLines import SmoothMeshLines

    b = m["board"]
    air = positive(params, "air_mm")
    res = positive(params, "mesh_mm")
    top = m["copper"][0]["z"] + m["copper"][0]["thickness"]
    tiers = {axis: [[], [], []] for axis in "xyz"}
    tiers["x"][0] += [b["min_x"] - air, b["min_x"], b["max_x"], b["max_x"] + air]
    tiers["y"][0] += [b["min_y"] - air, b["min_y"], b["max_y"], b["max_y"] + air]
    tiers["z"][0] += [-air, top + air]
    for c in m["copper"]:
        tiers["z"][0].append(geometry.sheet_z(m, c["name"]))
    for d in m["dielectrics"]:
        tiers["z"][0] += [d["z"] + d["thickness"] * k / 4 for k in range(5)]
    for start, stop in boxes:
        tiers["x"][0] += [start[0], stop[0]]
        tiers["y"][0] += [start[1], stop[1]]
    for sh in m["shapes"]:
        xs = [p[0] for p in sh["points"]]
        ys = [p[1] for p in sh["points"]]
        tiers["x"][1].append((min(xs) + max(xs)) / 2)
        tiers["y"][1].append((min(ys) + max(ys)) / 2)
        tiers["x"][2] += [min(xs), max(xs)]
        tiers["y"][2] += [min(ys), max(ys)]
    for h in m["holes"]:
        tiers["x"][1].append(h["x"])
        tiers["y"][1].append(h["y"])
        for q in h.get("slot", []):
            tiers["x"][2].append(q[0])
            tiers["y"][2].append(q[1])
    grid = csx.GetGrid()
    grid.SetDeltaUnit(1e-3)
    cells = 1
    for axis in "xyz":
        fixed = merge([[round(v, 6) for v in t] for t in tiers[axis]], res / 4)
        pts = SmoothMeshLines(fixed, res, 1.4)
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


# The TDR's time steps per 1 / (2 f_stop): its spectrum is padded with
# zeros, so the step is drawn smoothly, not every 1 / (2 f_stop).
TDR_PAD = 16


def tdr(s11, df, z0):
    """The TDR impedance from S11 on 0, df, 2 df, ...: the reflection of a
    step (a Hann window over the band sets its rise time), as
    Z0 (1 + r) / (1 - r). Returns (time in s, impedance in ohm), from a
    little before t = 0, so that a reflection at t = 0 rises in full."""
    import numpy as np

    n = len(s11)
    window = np.hanning(2 * n)[n:]
    m = 2 * (n - 1) * TDR_PAD
    impulse = np.fft.irfft(s11 * window, m)
    # The impulse is spread both sides of t = 0, its early half wrapped to
    # the array's end: put t = 0 in the middle and integrate from there.
    rolled = np.roll(impulse, m // 2)
    step = np.clip(np.cumsum(rolled), -0.999, 0.999)
    dt = 1.0 / (m * df)
    t = (np.arange(m) - m // 2) * dt
    # From the window's own width before t = 0.
    lead = int(round(2.0 / (2 * (n - 1) * df) / dt))
    keep = slice(m // 2 - lead, m)
    return t[keep], z0 * (1 + step[keep]) / (1 - step[keep])


def rise_time(n, df):
    """The 10 % to 90 % rise time of the TDR's step, for its band."""
    import numpy as np

    t, z = tdr(np.ones(n, dtype=complex), df, 1.0)
    r = (z - 1) / (z + 1)
    return float(t[np.argmax(r >= 0.9 * r[-1])] - t[np.argmax(r >= 0.1 * r[-1])])


# Where matplotlib keeps the fonts it ships with, in any copy of it.
OWN_FONTS = os.sep + os.path.join("matplotlib", "mpl-data", "fonts") + os.sep


def _pyplot():
    """matplotlib's pyplot, on its own fonts only. Building its font list,
    matplotlib runs the host's fc-list and reads the host's font
    directories; without fc-list on the PATH it runs nothing, and keeping
    only the fonts it ships with makes the plots the same on every host."""
    import matplotlib

    if not getattr(_pyplot, "done", False):
        os.environ["PATH"] = ""
        matplotlib.use("Agg")
        from matplotlib import font_manager

        fm = font_manager.fontManager
        fm.ttflist = [f for f in fm.ttflist if OWN_FONTS in f.fname]
        fm.afmlist = [f for f in fm.afmlist if OWN_FONTS in f.fname]
        fm._findfont_cached.cache_clear()
        _pyplot.done = True
    from matplotlib import pyplot as plt

    return plt


def plot(path, title, xlabel, ylabel, series):
    """A line plot, written without a time or version in its metadata."""
    plt = _pyplot()

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
    z0 = positive(params, "z0")
    positive(params, "mesh_mm")
    positive(params, "air_mm")
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
    t_end = 0.0
    notes = []
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
        boxes = []
        for p in ports:
            start, stop, note = port_box(m, p)
            boxes.append((start, stop))
            if note and i == 0:
                notes.append(note)
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
            # The time the run simulated.
            t_end = float(lumped[i].u_time[-1])
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
        # Up to the time the run simulated: past it, nothing was solved.
        keep = t <= t_end
        tdr_series.append(("Z %s" % p, t[keep] * 1e12, z[keep]))
        tdr_ranges[p] = (float(z[keep].min()), float(z[keep].max()))
    rise = rise_time(tdr_n, tdr_df)
    plot(os.path.join(out_dir, "tdr.png"), "TDR impedance (rise time %.0f ps, 10-90 %%)" % (rise * 1e12),
         "time (ps)", "Z (ohm)", tdr_series)

    with open(os.path.join(out_dir, "report.md"), "w") as f:
        w = f.write
        w("# %s\n\n" % name)
        w("openEMS FDTD, %d ports, each excited in turn; copper as sheets.\n\n" % n)
        w("* Band: %.4g to %.4g GHz, %d points.\n" % (f_start / 1e9, f_stop / 1e9, points))
        w("* Mesh: %d cells, at most %g mm; %g mm of air, its outer 8 cells an absorbing boundary (PML).\n"
          % (cells, float(params["mesh_mm"]), float(params["air_mm"])))
        w("* Each run takes %d time steps.\n" % int(params["steps"]))
        w("* Ports: lumped, %g ohm, from the pad to the nearest copper layer with copper under it.\n" % z0)
        w("* TDR: a step of %.0f ps rise time (10 to 90 %%), over the %.0f ps each run simulated.\n"
          % (rise * 1e12, t_end * 1e12))
        for note in notes:
            w("* Note: %s.\n" % note)
        w("\n")
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
