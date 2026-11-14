<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Public API of hdlf_rules_em.

Load every rule from this file:

```starlark
load(
    "@hdlf_rules_em//em:defs.bzl",
    "determinism_test",
    "kicad_em_model",
    "kicad_em_sim",
    "swap_test",
    "tool_toolchain",
)
```

`kicad_em_model` makes a board's nets an openEMS model, and `kicad_em_sim`
solves it: S-parameters, impedance and TDR. `kicad_em_test` follows, as
https://github.com/filmil/bazel_rules_em/issues/1 lays out. This
file is the only public entry point; everything under `//em/private` may
change without notice.

<a id="kicad_em_model"></a>

## kicad_em_model

<pre>
load("@hdlf_rules_em//em:defs.bzl", "kicad_em_model")

kicad_em_model(<a href="#kicad_em_model-name">name</a>, <a href="#kicad_em_model-copper_thickness">copper_thickness</a>, <a href="#kicad_em_model-dielectrics">dielectrics</a>, <a href="#kicad_em_model-nets">nets</a>, <a href="#kicad_em_model-pcb">pcb</a>, <a href="#kicad_em_model-ports">ports</a>, <a href="#kicad_em_model-px_per_mm">px_per_mm</a>)
</pre>

A board's nets as an openEMS model, built from the board's copper.

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

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_em_model-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_em_model-copper_thickness"></a>copper_thickness |  Each copper layer's thickness, in mm: `"0.035"` or `"0.035mm"`. The default is 1 oz copper.   | String | optional |  `"0.035"`  |
| <a id="kicad_em_model-dielectrics"></a>dielectrics |  The dielectric layers, top first, one per gap between copper layers: `"<thickness>mm <epsilon_r> <loss tangent>"`, such as `"1.51mm 4.5 0.02"`.   | List of strings | required |  |
| <a id="kicad_em_model-nets"></a>nets |  The nets to model, by the board's names. The reference net (`GND`) too.   | List of strings | required |  |
| <a id="kicad_em_model-pcb"></a>pcb |  The board: a `.kicad_pcb` (KiCad 7 to 9).   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="kicad_em_model-ports"></a>ports |  Ports: a name to a pad, `REF.PAD`, such as `{"P1": "J1.A6"}`. Each pad's net must be one of `nets`, and its name its own (a pad name that several pads share fails). A through-hole pad's port is on its first layer; `REF.PAD@B.Cu` puts it on another.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: String -> String</a> | optional |  `{}`  |
| <a id="kicad_em_model-px_per_mm"></a>px_per_mm |  The picture's pixels per mm: 1 to 100.   | Integer | optional |  `20`  |


<a id="kicad_em_sim"></a>

## kicad_em_sim

<pre>
load("@hdlf_rules_em//em:defs.bzl", "kicad_em_sim")

kicad_em_sim(<a href="#kicad_em_sim-name">name</a>, <a href="#kicad_em_sim-air_mm">air_mm</a>, <a href="#kicad_em_sim-f_start">f_start</a>, <a href="#kicad_em_sim-f_stop">f_stop</a>, <a href="#kicad_em_sim-mesh_mm">mesh_mm</a>, <a href="#kicad_em_sim-model">model</a>, <a href="#kicad_em_sim-points">points</a>, <a href="#kicad_em_sim-steps">steps</a>, <a href="#kicad_em_sim-threads">threads</a>, <a href="#kicad_em_sim-z0">z0</a>)
</pre>

Solves a kicad_em_model with openEMS: S-parameters, impedance and TDR.

```starlark
kicad_em_sim(
    name = "usb_sim",
    model = ":usb_model",  # kicad_em_model, with ports
    f_start = "100MHz",
    f_stop = "6GHz",
    tags = ["fdtd"],
)
```

It writes a directory `<name>/`:

* `<name>.s<N>p`: the S-parameters of the model's N ports, Touchstone 1.1
  (Hz, S, real and imaginary, R `z0`), the ports in name order;
* `s_params.png`: |S| in dB of every pair;
* `impedance.png`: each port's input impedance, real and imaginary;
* `tdr.png`: each port's TDR impedance, from its S11: the reflection of a
  step whose rise time, about 1 / `f_stop`, the report gives, over the time
  each run simulated;
* `report.md`: the setup, the mesh, S at the band's ends and middle, and
  each port's lowest and highest TDR impedance.

Each port is a lumped port of `z0` from its pad to the nearest copper layer,
going away from the pad's layer, with modelled copper under the pad: that
copper is the reference, so model it, such as `GND`. If no layer has, the
port ends on the adjacent layer, and the report says so. Every port is excited once, in its own run, so N
ports take N runs. The copper is drawn as sheets of no thickness, so the
mesh needs no cells as thin as the copper. The mesh has lines at each
copper shape's edges and centre, and is at most `mesh_mm`. `air_mm` of air
lies around the board, and its outer 8 cells absorb (PML), so it must be at
least 8 `mesh_mm`. Each run takes exactly `steps` time steps: openEMS
checks its own end criterion at wall-clock intervals, so stopping on it
would make the result depend on the machine's speed. Give enough steps for
the pulse to leave the board (the report of a run with more steps shows
whether the S-parameters still move). The dielectric's loss tangent is
applied at the band's centre.

openEMS runs its multithreaded engine with `threads` threads, so the same
model gives the same bytes. FDTD is slow: tag the target `fdtd`, and keep
its mesh and `steps` small in tests.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_em_sim-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_em_sim-air_mm"></a>air_mm |  The air around the board, in mm: at least 8 `mesh_mm`, the absorbing boundary's cells, and better 16, so the boundary is clear of the board's fields.   | String | optional |  `"4"`  |
| <a id="kicad_em_sim-f_start"></a>f_start |  The band's start: a number and Hz, kHz, MHz or GHz.   | String | optional |  `"100MHz"`  |
| <a id="kicad_em_sim-f_stop"></a>f_stop |  The band's end, which also sets the excitation's width and the TDR's rise time.   | String | optional |  `"6GHz"`  |
| <a id="kicad_em_sim-mesh_mm"></a>mesh_mm |  The largest mesh cell, in mm. Finer is slower and closer.   | String | optional |  `"0.25"`  |
| <a id="kicad_em_sim-model"></a>model |  The model (`kicad_em_model`), with ports.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="kicad_em_sim-points"></a>points |  The number of frequencies in the band, for the Touchstone file and the plots.   | Integer | optional |  `101`  |
| <a id="kicad_em_sim-steps"></a>steps |  The time steps of each run, exactly: 100 or more.   | Integer | optional |  `20000`  |
| <a id="kicad_em_sim-threads"></a>threads |  openEMS's thread count, fixed.   | Integer | optional |  `2`  |
| <a id="kicad_em_sim-z0"></a>z0 |  The ports' reference impedance, in ohm.   | String | optional |  `"50"`  |


<a id="tool_toolchain"></a>

## tool_toolchain

<pre>
load("@hdlf_rules_em//em:defs.bzl", "tool_toolchain")

tool_toolchain(<a href="#tool_toolchain-name">name</a>, <a href="#tool_toolchain-tools">tools</a>)
</pre>

A toolchain that offers tools by name, for `run_tool`.

Example, a toolchain for a tool of your own:

```starlark
tool_toolchain(
    name = "my_tool_impl",
    tools = {"//third_party/my_tool": "my_tool"},
)

toolchain(
    name = "my_tool",
    toolchain = ":my_tool_impl",
    toolchain_type = "//my:toolchain_type",
)
```

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="tool_toolchain-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="tool_toolchain-tools"></a>tools |  Executable targets, each mapped to the name rules use for it.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: Label -> String</a> | required |  |


<a id="determinism_test"></a>

## determinism_test

<pre>
load("@hdlf_rules_em//em:defs.bzl", "determinism_test")

determinism_test(<a href="#determinism_test-name">name</a>, <a href="#determinism_test-target">target</a>, <a href="#determinism_test-kwargs">**kwargs</a>)
</pre>

Checks that two builds of `target` give byte-identical outputs.

The target is built twice in one build, under two configurations that
differ only in a setting no rule reads, so every action runs twice.
The test compares the SHA-256 of every output file, directories walked,
and fails on any difference (spec D1).

Two runs in the same second can hide a time stamp with a one-second
resolution, so pair this with a header scan where a format has one. CI
also builds every determinism_test target in two fresh output bases and
compares the results.

Example:

```starlark
determinism_test(
    name = "microstrip_model_determinism_test",
    target = ":microstrip_model",
)
```


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="determinism_test-name"></a>name |  the test's name.   |  none |
| <a id="determinism_test-target"></a>target |  the target whose outputs must not change between builds.   |  none |
| <a id="determinism_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


<a id="swap_test"></a>

## swap_test

<pre>
load("@hdlf_rules_em//em:defs.bzl", "swap_test")

swap_test(<a href="#swap_test-name">name</a>, <a href="#swap_test-targets">targets</a>, <a href="#swap_test-kwargs">**kwargs</a>)
</pre>

Builds targets with every tool swapped for a stub, and checks them.

The targets are built with `--@hdlf_rules_em//em/toolchains:use_stub`,
so each tool toolchain that honors the flag resolves to the stub tool.
The stub writes a record into every output, files and directories
alike. The test fails if any output file of the targets lacks the
record: some action made it without a toolchain tool.

Example:

```starlark
swap_test(
    name = "swap_test",
    targets = [":microstrip_model", ":microstrip_sim"],
)
```


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="swap_test-name"></a>name |  the test's name.   |  none |
| <a id="swap_test-targets"></a>targets |  targets whose outputs must all come from the stub.   |  none |
| <a id="swap_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


