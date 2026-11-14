# SPDX-License-Identifier: Apache-2.0
"""kicad_em_sim: S-parameters, impedance and TDR of a model (issue #1, step 3)."""

load(":model.bzl", "EmModelInfo")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

TOOLCHAIN_TYPE = Label("//em:toolchain_type")

EmSimInfo = provider(
    doc = "A kicad_em_sim's outputs, for kicad_em_test.",
    fields = {
        "dir": "File: the output directory.",
        "touchstone": "string: the Touchstone file's name in it, such as usb.s4p.",
        "ports": "list of string: the ports, in the Touchstone file's order.",
    },
)

# numpy's BLAS reads these; one thread keeps its sums in a fixed order.
_ONE_THREAD = {
    "MKL_NUM_THREADS": "1",
    "OMP_NUM_THREADS": "1",
    "OPENBLAS_NUM_THREADS": "1",
}

def _kicad_em_sim_impl(ctx):
    model = ctx.attr.model[EmModelInfo]
    if not model.ports:
        fail("%s: the model %s has no ports: give kicad_em_model's ports" % (ctx.label, ctx.attr.model.label))
    if ctx.attr.points < 2:
        fail("%s: points %d: want 2 or more" % (ctx.label, ctx.attr.points))
    if ctx.attr.steps < 100:
        fail("%s: steps %d: want 100 or more" % (ctx.label, ctx.attr.steps))
    if ctx.attr.threads < 1:
        fail("%s: threads %d: want 1 or more" % (ctx.label, ctx.attr.threads))
    out = ctx.actions.declare_directory(ctx.label.name)
    params = {
        "model": model.json.path,
        "f_start": ctx.attr.f_start,
        "f_stop": ctx.attr.f_stop,
        "points": ctx.attr.points,
        "mesh_mm": ctx.attr.mesh_mm,
        "air_mm": ctx.attr.air_mm,
        "z0": ctx.attr.z0,
        "steps": ctx.attr.steps,
        "name": ctx.label.name,
    }
    run_tool(
        ctx,
        TOOLCHAIN_TYPE,
        "openems",
        arguments = [
            "--script",
            ctx.file._script.path,
            "--out",
            out.path,
            "--threads",
            str(ctx.attr.threads),
            "--params",
            json.encode(params),
        ],
        inputs = [ctx.file._script, ctx.file._geometry, model.json],
        outputs = [out],
        mnemonic = "EmSim",
        progress_message = "Solving %{label} with openEMS",
        env = _ONE_THREAD,
        # matplotlib writes its cache there.
        home_env = ["MPLCONFIGDIR"],
    )
    n = len(model.ports)
    return [
        DefaultInfo(files = depset([out])),
        EmSimInfo(dir = out, touchstone = "%s.s%dp" % (ctx.label.name, n), ports = model.ports),
    ]

kicad_em_sim = rule(
    implementation = _kicad_em_sim_impl,
    doc = """Solves a kicad_em_model with openEMS: S-parameters, impedance and TDR.

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
  each run simulated. A feature whose delay is not well above that rise
  time reads closer to the ports' impedance than it is: a 16 mm line of
  about 145 ohm reads about 115 ohm with a 161 ps step, and a 30 mm one of
  about 128 ohm reads 125 ohm with a 97 ps step. Raise `f_stop` to see
  shorter features;
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
""",
    attrs = {
        "model": attr.label(
            doc = "The model (`kicad_em_model`), with ports.",
            mandatory = True,
            providers = [EmModelInfo],
        ),
        "f_start": attr.string(
            doc = "The band's start: a number and Hz, kHz, MHz or GHz.",
            default = "100MHz",
        ),
        "f_stop": attr.string(
            doc = "The band's end, which also sets the excitation's width and the TDR's rise time.",
            default = "6GHz",
        ),
        "points": attr.int(
            doc = "The number of frequencies in the band, for the Touchstone file and the plots.",
            default = 101,
        ),
        "mesh_mm": attr.string(
            doc = "The largest mesh cell, in mm. Finer is slower and closer.",
            default = "0.25",
        ),
        "air_mm": attr.string(
            doc = "The air around the board, in mm: at least 8 `mesh_mm`, the absorbing " +
                  "boundary's cells, and better 16, so the boundary is clear of the board's fields.",
            default = "4",
        ),
        "z0": attr.string(
            doc = "The ports' reference impedance, in ohm.",
            default = "50",
        ),
        "steps": attr.int(
            doc = "The time steps of each run, exactly: 100 or more.",
            default = 20000,
        ),
        "threads": attr.int(
            doc = "openEMS's thread count, fixed.",
            default = 2,
        ),
        "_script": attr.label(
            default = Label("//em/private/model:sim.py"),
            allow_single_file = [".py"],
        ),
        "_geometry": attr.label(
            default = Label("//em/private/model:geometry.py"),
            allow_single_file = [".py"],
        ),
    },
    toolchains = [TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
