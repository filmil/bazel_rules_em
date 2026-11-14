# SPDX-License-Identifier: Apache-2.0
"""kicad_em_model's geometry: model.json (pcbmodel's) as CSXCAD XML.

The openEMS runner runs it (simulate(out_dir, params, run)); it builds
geometry only and never calls run, so nothing is solved. params:
{"model": "<path of model.json>"}. It writes out_dir/model.xml, as
geometry.build draws it with the copper's thickness. The grid's unit is
the mm (DeltaUnit 1e-3).

model.json's ports are not in model.xml: a port is part of a
simulation, which kicad_em_sim sets up.
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import geometry  # noqa: E402


def simulate(out_dir, params, run):
    from CSXCAD import ContinuousStructure

    with open(params["model"]) as f:
        m = json.load(f)
    csx = ContinuousStructure()
    csx.GetGrid().SetDeltaUnit(1e-3)
    geometry.build(csx, m)
    csx.Write2XML(out_dir + "/model.xml")
