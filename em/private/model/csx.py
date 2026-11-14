# SPDX-License-Identifier: Apache-2.0
"""kicad_em_model's geometry: model.json (pcbmodel's) as CSXCAD XML.

The openEMS runner runs it (simulate(out_dir, params, run)); it builds
geometry only and never calls run, so nothing is solved. params:
{"model": "<path of model.json>"}. It writes out_dir/model.xml:

* each dielectric layer, a box over the board's outline box, as a
  material with its relative permittivity (the loss tangent needs a
  frequency, so kicad_em_sim applies it); an inner copper layer's slab is
  filled with the dielectric above it, so the gaps between its copper are
  not air;
* each net's copper as a metal property named "net:<net>": every shape
  extruded through its layer's thickness, and every plated hole (a
  cylinder of its drill's radius, or a slot's outline) through the layers
  it joins.

model.json's ports are not in model.xml: a port is part of a
simulation, which kicad_em_sim sets up.

The grid's unit is the mm (DeltaUnit 1e-3), and each property's colour
is fixed (CSXCAD otherwise picks colours at random).
"""

# Fixed colours: the dielectric, then the nets in turn.
DIELECTRIC = "#67c669"
NETS = ["#c83c2c", "#2c5cc8", "#d08c1c", "#7c3c8c", "#1c9c9c", "#8c8c2c"]

import json


def simulate(out_dir, params, run):
    from CSXCAD import ContinuousStructure

    with open(params["model"]) as f:
        m = json.load(f)
    csx = ContinuousStructure()
    csx.GetGrid().SetDeltaUnit(1e-3)
    board = m["board"]
    copper = m["copper"]
    for i, d in enumerate(m["dielectrics"]):
        mat = csx.AddMaterial(d["name"].replace(" ", "_"), epsilon=d["epsilon_r"])
        mat.SetColor(DIELECTRIC, 123)
        mat.AddBox(
            [board["min_x"], board["min_y"], d["z"]],
            [board["max_x"], board["max_y"], d["z"] + d["thickness"]],
            priority=1,
        )
        # The copper layer below this dielectric, if it is an inner one:
        # its slab is this dielectric where it has no copper.
        below = copper[i + 1]
        if i + 1 < len(copper) - 1:
            mat.AddBox(
                [board["min_x"], board["min_y"], below["z"]],
                [board["max_x"], board["max_y"], below["z"] + below["thickness"]],
                priority=1,
            )
    z = {c["name"]: (c["z"], c["thickness"]) for c in m["copper"]}
    metals = {}
    for i, net in enumerate(m["nets"]):
        metals[net] = csx.AddMetal("net:" + net)
        metals[net].SetColor(NETS[i % len(NETS)], 255)
    for s in m["shapes"]:
        z0, t = z[s["layer"]]
        xs = [p[0] for p in s["points"]]
        ys = [p[1] for p in s["points"]]
        metals[s["net"]].AddLinPoly([xs, ys], "z", z0, t, priority=10)
    for h in m["holes"]:
        zs = [z[l][0] for l in h["layers"]] + [z[l][0] + z[l][1] for l in h["layers"]]
        if "slot" in h:
            xs = [p[0] for p in h["slot"]]
            ys = [p[1] for p in h["slot"]]
            metals[h["net"]].AddLinPoly([xs, ys], "z", min(zs), max(zs) - min(zs), priority=10)
        else:
            metals[h["net"]].AddCylinder(
                [h["x"], h["y"], min(zs)], [h["x"], h["y"], max(zs)], h["drill"] / 2, priority=10
            )
    csx.Write2XML(out_dir + "/model.xml")
