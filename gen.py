"""Writes the pcb-em-intuition test boards: <case>/board.kicad_pcb and
<case>/spec.json (nets, ports, dielectrics) for pcbmodel and sim.py."""

import json
import os
import sys

OUT = sys.argv[1]
CU = 0.035
L2 = ["1.53mm 4.3 0.02"]
L4 = ["0.2mm 4.3 0.02", "1.06mm 4.3 0.02", "0.2mm 4.3 0.02"]


class Board:
    def __init__(self, layers, box):
        self.layers = layers  # 2 or 4
        self.box = box  # x0, y0, x1, y1
        self.nets = ["GND"]
        self.items = []
        self.fps = []
        self.n = 0

    def net(self, name):
        if name not in self.nets:
            self.nets.append(name)
        return self.nets.index(name) + 1

    def track(self, pts, w, net, layer="F.Cu"):
        k = self.net(net)
        for a, b in zip(pts, pts[1:]):
            self.items.append('\t(segment (start %g %g) (end %g %g) (width %g) (layer "%s") (net %d))'
                              % (a[0], a[1], b[0], b[1], w, layer, k))

    def arc(self, a, m, b, w, net, layer="F.Cu"):
        k = self.net(net)
        self.items.append('\t(arc (start %g %g) (mid %g %g) (end %g %g) (width %g) (layer "%s") (net %d))'
                          % (a[0], a[1], m[0], m[1], b[0], b[1], w, layer, k))

    def via(self, x, y, net, size=0.6, drill=0.3):
        k = self.net(net)
        self.items.append('\t(via (at %g %g) (size %g) (drill %g) (layers "F.Cu" "B.Cu") (net %d))'
                          % (x, y, size, drill, k))

    def pour(self, layer, rects, net="GND"):
        k = self.net(net)
        for r in rects:
            x0, y0, x1, y1 = r
            self.items.append('\t(zone (net %d) (net_name "%s") (layer "%s")\n\t\t(filled_polygon (layer "%s")\n'
                              '\t\t\t(pts (xy %g %g) (xy %g %g) (xy %g %g) (xy %g %g))))'
                              % (k, net, layer, layer, x0, y0, x1, y0, x1, y1, x0, y1))

    def plane(self, layer, hole=None, net="GND"):
        """A full plane, or one with a square hole (x, y, half)."""
        x0, y0, x1, y1 = self.box
        if hole is None:
            self.pour(layer, [self.box], net)
            return
        hx, hy, h = hole
        self.pour(layer, [(x0, y0, hx - h, y1), (hx + h, y0, x1, y1),
                          (hx - h, y0, hx + h, hy - h), (hx - h, hy + h, hx + h, y1)], net)

    def pad(self, ref, x, y, w, net, layer="F.Cu", horizontal=True):
        """A port pad: 0.5 mm along the trace, w across it."""
        k = self.net(net)
        size = (0.5, w) if horizontal else (w, 0.5)
        self.fps.append('\t(footprint "Port" (layer "%s") (at %g %g)\n\t\t(property "Reference" "%s")\n'
                        '\t\t(pad "1" smd rect (at 0 0) (size %g %g) (layers "%s") (net %d "%s")))'
                        % (layer, x, y, ref, size[0], size[1], layer, k, net))

    def text(self):
        names = ["F.Cu", "B.Cu"] if self.layers == 2 else ["F.Cu", "In1.Cu", "In2.Cu", "B.Cu"]
        ids = {"F.Cu": 0, "In1.Cu": 4, "In2.Cu": 6, "B.Cu": 2}
        out = ['(kicad_pcb\n\t(version 20241229)\n\t(generator "pcbnew")\n\t(general (thickness 1.6))\n\t(layers']
        for n in names:
            out.append('\t\t(%d "%s" signal)' % (ids[n], n))
        out.append('\t\t(25 "Edge.Cuts" user)\n\t)\n\t(net 0 "")')
        for i, n in enumerate(self.nets):
            out.append('\t(net %d "%s")' % (i + 1, n))
        out += self.fps + self.items
        x0, y0, x1, y1 = self.box
        out.append('\t(gr_rect (start %g %g) (end %g %g) (layer "Edge.Cuts"))\n)\n' % (x0, y0, x1, y1))
        return "\n".join(out)


def write(name, b, ports, nets, note):
    d = os.path.join(OUT, name)
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "board.kicad_pcb"), "w") as f:
        f.write(b.text())
    with open(os.path.join(d, "spec.json"), "w") as f:
        json.dump({"nets": nets, "ports": ports, "dielectrics": L2 if b.layers == 2 else L4, "note": note}, f,
                  indent=1)


def straight(name, w, layers, note, extra=None):
    """A 30 mm trace from x 1 to 31, ports at both ends."""
    half = 10 if w > 1 else 6
    b = Board(layers, (0, -half, 32, half))
    b.track([(1, 0), (31, 0)], w, "SIG")
    b.pad("J1", 1, 0, w, "SIG")
    b.pad("J2", 31, 0, w, "SIG")
    if layers == 2:
        b.plane("B.Cu")
    else:
        b.plane("In1.Cu")
        b.plane("In2.Cu")
    nets = ["SIG", "GND"]
    if extra:
        nets = extra(b, nets)
    write(name, b, {"P1": "J1.1", "P2": "J2.1"}, nets, note)


# S1, S2: microstrips on 2 layers.
straight("S1_microstrip_3mm", 3.0, 2, "3.0 mm microstrip over solid GND, 2-layer 1.6 mm")
straight("S2_microstrip_0.3mm", 0.3, 2, "0.3 mm microstrip over solid GND, 2-layer 1.6 mm")


# S3: bends. 15 mm along X, the corner, 15 mm along Y.
def bend(name, kind):
    w = 3.0
    b = Board(2, (-1, -10, 22, 13))
    if kind == "90":
        b.track([(1, -8), (16, -8), (16, 7)], w, "SIG")
    elif kind == "mitre":
        c = 3.0
        b.track([(1, -8), (16 - c, -8), (16, -8 + c), (16, 7)], w, "SIG")
    else:
        r = 3.0
        b.track([(1, -8), (16 - r, -8)], w, "SIG")
        b.arc((16 - r, -8), (16 - r + r * 0.7071068, -8 + r - r * 0.7071068), (16, -8 + r), w, "SIG")
        b.track([(16, -8 + r), (16, 7)], w, "SIG")
    b.pad("J1", 1, -8, w, "SIG")
    b.pad("J2", 16, 7, w, "SIG", horizontal=False)
    b.plane("B.Cu")
    write(name, b, {"P1": "J1.1", "P2": "J2.1"}, ["SIG", "GND"],
          "3.0 mm microstrip with a %s corner at mid-length, 2-layer" % kind)


bend("S3a_bend_90", "90")
bend("S3b_bend_mitre", "mitre")
bend("S3c_bend_arc", "arc")


# S4: a 10 mm open stub at mid-length.
def stub(b, nets):
    b.track([(16, 0), (16, 10)], 3.0, "SIG")
    return nets


def s4():
    w = 3.0
    b = Board(2, (0, -10, 32, 14))
    b.track([(1, 0), (31, 0)], w, "SIG")
    b.track([(16, 0), (16, 10)], w, "SIG")
    b.pad("J1", 1, 0, w, "SIG")
    b.pad("J2", 31, 0, w, "SIG")
    b.plane("B.Cu")
    write("S4_open_stub_10mm", b, {"P1": "J1.1", "P2": "J2.1"}, ["SIG", "GND"],
          "3.0 mm microstrip with a 10 mm open stub (centre line) at mid-length, 2-layer")


s4()


# S5: a 2 mm x 20 mm slot in GND across the trace at mid-length.
def s5():
    w = 3.0
    b = Board(2, (0, -15, 32, 15))
    b.track([(1, 0), (31, 0)], w, "SIG")
    b.pad("J1", 1, 0, w, "SIG")
    b.pad("J2", 31, 0, w, "SIG")
    b.pour("B.Cu", [(0, -15, 15, 15), (17, -15, 32, 15), (15, -15, 17, -10), (15, 10, 17, 15)])
    write("S5_split_plane_slot", b, {"P1": "J1.1", "P2": "J2.1"}, ["SIG", "GND"],
          "3.0 mm microstrip over GND with a 2 mm wide, 20 mm long slot across it at mid-length, 2-layer")


s5()


# S6: coplanar GND pour, 0.3 mm gap.
def s6(name, mode):
    w = 3.0
    g = 0.3
    b = Board(2, (0, -10, 32, 10))
    b.track([(1, 0), (31, 0)], w, "SIG")
    b.pad("J1", 1, 0, w, "SIG")
    b.pad("J2", 31, 0, w, "SIG")
    b.plane("B.Cu")
    top = "GND" if mode != "floating" else "FLOAT"
    e = w / 2 + g
    b.pour("F.Cu", [(0, e, 32, 10), (0, -10, 32, -e)], net=top)
    nets = ["SIG", "GND"] + (["FLOAT"] if top == "FLOAT" else [])
    if mode == "corners":
        for x in (0.5, 31.5):
            for y in (e + 0.5, -e - 0.5):
                b.via(x, y, "GND")
    if mode == "stitched":
        x = 1.0
        while x <= 31.0 + 1e-9:
            for y in (e + 0.5, -e - 0.5):
                b.via(x, y, "GND")
            x += 2.0
    note = {"corners": "grounded by one via at each of its four corners",
            "stitched": "stitched to GND by vias every 2 mm along both edges",
            "floating": "not connected to anything (floating)"}[mode]
    write(name, b, {"P1": "J1.1", "P2": "J2.1"}, nets,
          "3.0 mm trace with a coplanar pour 0.3 mm away on both sides, the pour %s; solid GND below, 2-layer" % note)


s6("S6a_cpwg_corner_vias", "corners")
s6("S6b_cpwg_stitched_2mm", "stitched")
s6("S6c_cpwg_floating", "floating")


# S7: crosstalk, two 0.3 mm traces, 4-layer.
def s7(n):
    w = 0.3
    gap = n * w
    y = (gap + w) / 2
    b = Board(4, (0, -6, 32, 6))
    b.track([(1, y), (31, y)], w, "AGG")
    b.track([(1, -y), (31, -y)], w, "VIC")
    b.pad("J1", 1, y, w, "AGG")
    b.pad("J2", 31, y, w, "AGG")
    b.pad("J3", 1, -y, w, "VIC")
    b.pad("J4", 31, -y, w, "VIC")
    b.plane("In1.Cu")
    b.plane("In2.Cu")
    write("S7_crosstalk_gap_%dW" % n, b, {"P1": "J1.1", "P2": "J2.1", "P3": "J3.1", "P4": "J4.1"},
          ["AGG", "VIC", "GND"],
          "two 0.3 mm traces, %g mm edge to edge (%dW), 0.2 mm over GND, 4-layer; P1/P2 aggressor near/far, "
          "P3/P4 victim near/far: NEXT = S31, FEXT = S41" % (gap, n))


for n in (1, 2, 3):
    s7(n)

# S8: 4-layer microstrip.
straight("S8_microstrip_4layer_0.36mm", 0.36, 4, "0.36 mm microstrip, 0.2 mm prepreg to GND, 4-layer")


# S9: a layer change through a via.
def s9(name, ret):
    w = 0.36
    b = Board(4, (0, -6, 32, 6))
    b.track([(1, 0), (16, 0)], w, "SIG", "F.Cu")
    b.via(16, 0, "SIG")
    b.track([(16, 0), (31, 0)], w, "SIG", "B.Cu")
    b.pad("J1", 1, 0, w, "SIG", "F.Cu")
    b.pad("J2", 31, 0, w, "SIG", "B.Cu")
    b.plane("In1.Cu", hole=(16, 0, 0.5))
    b.plane("In2.Cu", hole=(16, 0, 0.5))
    if ret:
        b.via(16, 1.0, "GND")
    write(name, b, {"P1": "J1.1", "P2": "J2.1"}, ["SIG", "GND"],
          "0.36 mm trace on F.Cu (over In1 GND) to a via at mid-length, then on B.Cu (over In2 GND), 4-layer; "
          "1 mm antipads in both planes; %s" % ("a GND via 1 mm away joins the planes" if ret
                                                else "no GND via: the planes join only by their capacitance"))


s9("S9a_via_with_return", True)
s9("S9b_via_no_return", False)


# S10: a differential pair, 4-layer.
def s10(name, skew):
    w, g = 0.2, 0.15
    y = (w + g) / 2
    b = Board(4, (0, -6, 32, 6))
    b.track([(1, y), (31, y)], w, "DP")
    if skew:
        b.track([(1, -y), (15, -y), (15, -y - 1), (17, -y - 1), (17, -y), (31, -y)], w, "DN")
    else:
        b.track([(1, -y), (31, -y)], w, "DN")
    b.pad("J1", 1, y, w, "DP")
    b.pad("J2", 31, y, w, "DP")
    b.pad("J3", 1, -y, w, "DN")
    b.pad("J4", 31, -y, w, "DN")
    b.plane("In1.Cu")
    b.plane("In2.Cu")
    write(name, b, {"P1": "J1.1", "P2": "J2.1", "P3": "J3.1", "P4": "J4.1"}, ["DP", "DN", "GND"],
          "differential pair, 0.2 mm traces, 0.15 mm gap, 0.2 mm over GND, 4-layer; P1/P2 = D+ near/far, "
          "P3/P4 = D- near/far%s" % ("; D- 2 mm longer (a 1 mm jog at mid-length)" if skew else ""))


s10("S10a_diff_pair", False)
s10("S10b_diff_pair_skew_2mm", True)
