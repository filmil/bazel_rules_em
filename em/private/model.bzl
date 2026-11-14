# SPDX-License-Identifier: Apache-2.0
"""kicad_em_model: a board's nets as an openEMS model (issue #1, step 2)."""

load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

TOOLCHAIN_TYPE = Label("//em:toolchain_type")

EmModelInfo = provider(
    doc = "A kicad_em_model's outputs, for kicad_em_sim.",
    fields = {
        "json": "File: model.json, the nets' copper, holes, ports and stackup.",
        "csx": "File: a directory holding model.xml, the CSXCAD geometry.",
        "png": "File: the picture of the model.",
        "ports": "list of string: the ports' names, sorted.",
    },
)

# numpy's BLAS reads these; one thread keeps its sums in a fixed order.
_ONE_THREAD = {
    "MKL_NUM_THREADS": "1",
    "OMP_NUM_THREADS": "1",
    "OPENBLAS_NUM_THREADS": "1",
}

def _kicad_em_model_impl(ctx):
    if not ctx.attr.nets:
        fail("%s: nets names no net to model" % ctx.label)
    if not ctx.attr.dielectrics:
        fail("%s: dielectrics is empty: give one per gap between copper layers" % ctx.label)
    if ctx.attr.px_per_mm < 1 or ctx.attr.px_per_mm > 100:
        fail("%s: px_per_mm %d: want 1 to 100" % (ctx.label, ctx.attr.px_per_mm))
    for name, pad in ctx.attr.ports.items():
        ref, _, pin = pad.partition("@")[0].partition(".")
        if not ref or not pin or "=" in name or not name:
            fail("%s: port %r = %r: want a name and a pad REF.PAD, such as {\"P1\": \"J1.A6\"}" % (ctx.label, name, pad))
    js = ctx.actions.declare_file(ctx.label.name + ".json")
    png = ctx.actions.declare_file(ctx.label.name + ".png")
    csx = ctx.actions.declare_directory(ctx.label.name + ".csx")

    args = ctx.actions.args()
    args.add("--pcb", ctx.file.pcb)
    args.add_all(ctx.attr.nets, before_each = "--net")
    args.add_all(["%s=%s" % (n, ctx.attr.ports[n]) for n in sorted(ctx.attr.ports)], before_each = "--port")
    args.add("--copper", ctx.attr.copper_thickness)
    args.add_all(ctx.attr.dielectrics, before_each = "--dielectric")
    args.add("--px_per_mm", str(ctx.attr.px_per_mm))
    args.add("--json", js)
    args.add("--png", png)
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "pcbmodel",
        arguments = [args],
        inputs = [ctx.file.pcb],
        outputs = [js, png],
        mnemonic = "EmModel",
        progress_message = "Reading the copper of %{label}",
    )
    run_tool(
        ctx,
        TOOLCHAIN_TYPE,
        "openems",
        arguments = [
            "--script",
            ctx.file._script.path,
            "--out",
            csx.path,
            "--params",
            json.encode({"model": js.path}),
        ],
        inputs = [ctx.file._script, ctx.file._geometry, js],
        outputs = [csx],
        mnemonic = "EmGeometry",
        progress_message = "Writing the CSXCAD geometry of %{label}",
        env = _ONE_THREAD,
        # matplotlib, which CSXCAD imports, writes a cache there.
        home_env = ["MPLCONFIGDIR"],
    )
    return [
        DefaultInfo(files = depset([js, png, csx])),
        EmModelInfo(json = js, csx = csx, png = png, ports = sorted(ctx.attr.ports.keys())),
    ]

kicad_em_model = rule(
    implementation = _kicad_em_model_impl,
    doc = """A board's nets as an openEMS model, built from the board's copper.

```starlark
kicad_em_model(
    name = "usb_model",
    pcb = ":pcb",                      # a .kicad_pcb
    nets = ["USB_DP", "USB_DM", "GND"],
    ports = {"P1": "J1.A6", "P2": "J1.A7", "P3": "U1.17", "P4": "U1.18"},
    copper_thickness = "0.035",        # mm, each copper layer
    dielectrics = ["1.51mm 4.5 0.02"], # top first: thickness, epsilon_r, loss tangent
)
```

It writes:

* `<name>.json`: the model. Each net's copper (tracks, arcs, pads and
  filled zones, each a polygon on its layer), its plated holes (vias and
  through-hole pads), the ports, and the stackup with each layer's Z.
  The frame is mm: X as KiCad's, Y up (KiCad's Y negated), Z up from the
  bottom copper's bottom face. Numbers have 6 decimals.
* `<name>.csx/model.xml`: the model's geometry as CSXCAD XML, for
  kicad_em_sim and for viewing in AppCSXCAD. Each dielectric is a material
  box over the outline's box (an inner copper layer's slab is the
  dielectric above it where it has no copper), and each net is a metal
  named `net:<net>`: its shapes extruded through their layer, its holes as
  barrels of the drill's size. The ports are not in it: kicad_em_sim sets
  them up from the JSON. The loss tangent needs a frequency, so
  kicad_em_sim applies it too.
* `<name>.png`: a picture from the top. Top copper is red, bottom copper
  blue (purple where both), inner copper green, holes dark, and ports amber
  rings.

The geometry comes from the `.kicad_pcb` itself, not from its Gerbers (as
gerber2ems does). The board file keeps each shape's net, so the model
names its nets and ports by the board's own names, and a port is a pad
by reference (`J1.A6`). Pads are drawn from their shape: circle, rect,
oval, roundrect, and custom (its anchor and its polygon, circle and
rectangle primitives; a chamfered pad fails). A via's pads are drawn on
each layer it joins, and an oval drill is a slot. Only the nets in `nets`
are in the model, so give the reference net (such as `GND`) too. A zone
saved without its fill fails: fill the zones in KiCad first. The board's
outline is the box of its Edge.Cuts items, footprints' included.

The board's file has no stackup unless KiCad's board setup gave it one,
so it is given here, one dielectric per gap between copper layers. The
model's thickness is the copper and dielectrics added up; the board's own
`(general (thickness))` is in the JSON beside it.

It runs no solver: the geometry is written by CSXCAD on the openEMS
toolchain, and nothing is solved. The same board gives the same bytes.
""",
    attrs = {
        "pcb": attr.label(
            doc = "The board: a `.kicad_pcb` (KiCad 7 to 9).",
            mandatory = True,
            allow_single_file = [".kicad_pcb"],
        ),
        "nets": attr.string_list(
            doc = "The nets to model, by the board's names. The reference net (`GND`) too.",
            mandatory = True,
        ),
        "ports": attr.string_dict(
            doc = "Ports: a name to a pad, `REF.PAD`, such as `{\"P1\": \"J1.A6\"}`. Each " +
                  "pad's net must be one of `nets`, and its name its own (a pad name that " +
                  "several pads share fails). A through-hole pad's port is on its first " +
                  "layer; `REF.PAD@B.Cu` puts it on another.",
        ),
        "copper_thickness": attr.string(
            doc = "Each copper layer's thickness, in mm: `\"0.035\"` or `\"0.035mm\"`. The " +
                  "default is 1 oz copper.",
            default = "0.035",
        ),
        "dielectrics": attr.string_list(
            doc = "The dielectric layers, top first, one per gap between copper layers: " +
                  "`\"<thickness>mm <epsilon_r> <loss tangent>\"`, such as `\"1.51mm 4.5 0.02\"`.",
            mandatory = True,
        ),
        "px_per_mm": attr.int(
            doc = "The picture's pixels per mm: 1 to 100.",
            default = 20,
        ),
        "_script": attr.label(
            default = Label("//em/private/model:csx.py"),
            allow_single_file = [".py"],
        ),
        "_geometry": attr.label(
            default = Label("//em/private/model:geometry.py"),
            allow_single_file = [".py"],
        ),
    },
    toolchains = [TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
