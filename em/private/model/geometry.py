# SPDX-License-Identifier: Apache-2.0
"""model.json as CSXCAD geometry, for kicad_em_model's model.xml and for
kicad_em_sim's solver runs.

build(csx, m) adds to a ContinuousStructure:

* each dielectric layer, a box over the board's outline box, as a
  material with its relative permittivity, and, with a frequency, its
  loss tangent as a conductivity (kappa = 2 pi f eps0 epsilon_r tan d);
  an inner copper layer's slab is the dielectric above it, so the gaps
  between its copper are not air;
* each net's copper as a metal property named "net:<net>": every shape
  on its layer, and every plated hole (a cylinder of its drill's radius,
  or a slot's outline) through the layers it joins.

With sheets=False, copper has its thickness (model.xml). With
sheets=True, each copper layer is a sheet of no thickness on the face
that touches the dielectric (sheet_z): the solver's mesh then needs no
cells as thin as the copper, which would make its time step tiny.
"""

import math

# Fixed colours: the dielectric, then the nets in turn. CSXCAD otherwise
# picks colours at random.
DIELECTRIC = "#67c669"
NETS = ["#c83c2c", "#2c5cc8", "#d08c1c", "#7c3c8c", "#1c9c9c", "#8c8c2c"]
EPS0 = 8.8541878128e-12


def sheet_z(m, layer):
    """The Z of a copper layer's sheet: the face on the dielectric (the
    top layer's bottom, the bottom layer's top, an inner layer's middle)."""
    copper = m["copper"]
    for i, c in enumerate(copper):
        if c["name"] == layer:
            if i == 0:
                return c["z"]
            if i == len(copper) - 1:
                return c["z"] + c["thickness"]
            return c["z"] + c["thickness"] / 2
    raise ValueError("no copper layer %s" % layer)


def build(csx, m, sheets=False, frequency=None):
    """Adds model m's dielectrics and copper to csx (unit: mm)."""
    board = m["board"]
    copper = m["copper"]
    for i, d in enumerate(m["dielectrics"]):
        kw = {"epsilon": d["epsilon_r"]}
        if frequency:
            kw["kappa"] = 2 * math.pi * frequency * EPS0 * d["epsilon_r"] * d["loss_tangent"]
        mat = csx.AddMaterial(d["name"].replace(" ", "_"), **kw)
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
    z = {c["name"]: (c["z"], c["thickness"]) for c in copper}
    metals = {}
    for i, net in enumerate(m["nets"]):
        metals[net] = csx.AddMetal("net:" + net)
        metals[net].SetColor(NETS[i % len(NETS)], 255)
    for s in m["shapes"]:
        xs = [p[0] for p in s["points"]]
        ys = [p[1] for p in s["points"]]
        if sheets:
            metals[s["net"]].AddPolygon([xs, ys], "z", sheet_z(m, s["layer"]), priority=10)
        else:
            z0, t = z[s["layer"]]
            metals[s["net"]].AddLinPoly([xs, ys], "z", z0, t, priority=10)
    for h in m["holes"]:
        if sheets:
            zs = [sheet_z(m, l) for l in h["layers"]]
        else:
            zs = [z[l][0] for l in h["layers"]] + [z[l][0] + z[l][1] for l in h["layers"]]
        if "slot" in h:
            xs = [p[0] for p in h["slot"]]
            ys = [p[1] for p in h["slot"]]
            metals[h["net"]].AddLinPoly([xs, ys], "z", min(zs), max(zs) - min(zs), priority=10)
        else:
            metals[h["net"]].AddCylinder(
                [h["x"], h["y"], min(zs)], [h["x"], h["y"], max(zs)], h["drill"] / 2, priority=10
            )
