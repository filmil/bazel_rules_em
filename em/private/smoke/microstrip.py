# SPDX-License-Identifier: Apache-2.0
"""The smoke test's model: a 30 mm, 3 mm wide line over a ground plane on
1.6 mm FR4 (epsilon_r 4.3), between two 50 ohm lumped ports, excited with a
Gaussian pulse from 0 to 2 GHz. 6000 time steps, Mur boundaries.

It writes the ports' time-domain voltages and currents (port_ut_*,
port_it_*) and s21.txt, |S21| in dB at 20 frequencies.
"""

import os

import numpy as np
from CSXCAD import ContinuousStructure
from openEMS import openEMS


def simulate(out_dir, params, run):
    fdtd = openEMS(NrTS=6000, EndCriteria=0)
    fdtd.SetGaussExcite(1e9, 1e9)
    fdtd.SetBoundaryCond(["MUR"] * 6)
    csx = ContinuousStructure()
    fdtd.SetCSX(csx)
    mesh = csx.GetGrid()
    mesh.SetDeltaUnit(1e-3)
    # Mesh lines on every edge of the model, so nothing falls between them.
    mesh.AddLine("x", [float(x) for x in range(-20, 21)])
    mesh.AddLine("y", [y / 2 for y in range(-20, 21)])
    mesh.AddLine("z", [0.0, 0.4, 0.8, 1.2, 1.6, 2.4, 3.2, 4.4, 6.0])
    csx.AddMaterial("FR4", epsilon=4.3).AddBox([-20, -10, 0], [20, 10, 1.6])
    csx.AddMetal("gnd").AddBox([-20, -10, 0], [20, 10, 0], priority=10)
    csx.AddMetal("line").AddBox([-15, -1.5, 1.6], [15, 1.5, 1.6], priority=10)
    p1 = fdtd.AddLumpedPort(1, 50, [-15, 0, 0], [-15, 0, 1.6], "z", excite=1)
    p2 = fdtd.AddLumpedPort(2, 50, [15, 0, 0], [15, 0, 1.6], "z")
    run(fdtd, out_dir)
    f = np.linspace(1e8, 2e9, 20)
    for p in (p1, p2):
        p.CalcPort(out_dir, f)
    s21 = 20 * np.log10(np.abs(p2.uf_ref / p1.uf_inc))
    with open(os.path.join(out_dir, "s21.txt"), "w") as out:
        for fi, s in zip(f, s21):
            out.write("%.0f\t%.6f\n" % (fi, s))
