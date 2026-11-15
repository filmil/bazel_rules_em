# SPDX-License-Identifier: Apache-2.0
"""kicad_em_test: a simulation's S-parameters against limits (issue #1, step 4)."""

load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE")
load(":sim.bzl", "EmSimInfo")

def _kicad_em_test_impl(ctx):
    sim = ctx.attr.sim[EmSimInfo]
    checker = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers.emcheck
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(
        output = exe,
        target_file = checker.files_to_run.executable,
        is_executable = True,
    )
    for name in ctx.attr.expect_fail:
        if "," in name or not name:
            fail("%s: expect_fail %r: want a check's name" % (ctx.label, name))
    return [
        DefaultInfo(
            executable = exe,
            runfiles = ctx.runfiles(files = [sim.dir, ctx.file.limits]).merge(checker.default_runfiles),
        ),
        RunEnvironmentInfo(environment = {
            "EMCHECK_TOUCHSTONE": sim.dir.short_path + "/" + sim.touchstone,
            "EMCHECK_PORTS": ",".join(sim.ports),
            "EMCHECK_LIMITS": ctx.file.limits.short_path,
            "EMCHECK_EXPECT_FAIL": ",".join(ctx.attr.expect_fail),
        }),
    ]

kicad_em_test = rule(
    implementation = _kicad_em_test_impl,
    test = True,
    doc = """Checks a kicad_em_sim's S-parameters against limits, and fails on a violation.

```starlark
kicad_em_test(
    name = "usb_em_test",
    size = "small",
    sim = ":usb_sim",           # kicad_em_sim
    limits = "usb_limits.json",
    tags = ["fdtd"],            # it reads a solve
)
```

The limits file holds a list of named checks. Ports are the model's port
names; a pair is (positive, negative) at the same end of the line:

```json
{"checks": [
  {"name": "usb_diff_impedance", "kind": "impedance", "pair": ["P1", "P2"],
   "from_ps": 150, "to_ps": 400, "min_ohm": 76.5, "max_ohm": 103.5},
  {"name": "dp_impedance", "kind": "impedance", "port": "P1",
   "from_ps": 150, "to_ps": 400, "min_ohm": 40, "max_ohm": 60},
  {"name": "usb_loss", "kind": "insertion_loss",
   "from_pair": ["P1", "P2"], "to_pair": ["P3", "P4"], "up_to": "1GHz", "max_db": 1.0},
  {"name": "dp_loss", "kind": "insertion_loss", "from": "P1", "to": "P3",
   "up_to": "1GHz", "max_db": 1.0},
  {"name": "usb_return", "kind": "return_loss", "pair": ["P1", "P2"],
   "up_to": "1GHz", "min_db": 10},
  {"name": "usb_skew", "kind": "skew", "positive": ["P1", "P3"],
   "negative": ["P2", "P4"], "max_ps": 10}
]}
```

* `impedance`: the TDR's impedance, single-ended for a `port`, or
  differential for a `pair` (from Sdd11, against twice the ports' z0), must
  stay from `min_ohm` to `max_ohm` from `from_ps` to `to_ps`. The TDR is
  kicad_em_sim's: a Hann-windowed step over the band, rise about 1 /
  f_stop, so a feature shorter than that reads closer to the ports'
  impedance than it is. It needs the band's grid to start at its step
  (such as f_start 10MHz, f_stop 10GHz, points 1000), and fails otherwise.
* `insertion_loss`: the loss from `from` to `to` (or Sdd between pairs),
  at its worst up to `up_to`, must be at most `max_db`.
* `return_loss`: the loss of the reflection at a `port` (or Sdd11 of a
  `pair`), at its worst up to `up_to`, must be at least `min_db`.
* `skew`: the arrivals of a step through the `positive` and `negative`
  lines, each (near port, far port), at half of its final value, must
  differ by at most `max_ps`. The first edge arrives before any reflection,
  so a mismatch at the ports does not bend it, and both lines see the same
  one. It uses the whole band, and the grid rule of `impedance`.

The test prints a table of each check, what it measured, its limit and
its result, and writes it as `report.md` in its undeclared outputs.

`expect_fail` names the checks that this board must fail: a known-bad
board, which proves that the checks catch what they are for. The test then
fails if one of those passes, or if any other check fails.

The test reads a solve, so tag it `fdtd`, as its kicad_em_sim is.
""",
    attrs = {
        "sim": attr.label(
            doc = "The simulation (`kicad_em_sim`).",
            mandatory = True,
            providers = [EmSimInfo],
        ),
        "limits": attr.label(
            doc = "The limits: a JSON file of named checks.",
            mandatory = True,
            allow_single_file = [".json"],
        ),
        "expect_fail": attr.string_list(
            doc = "The checks this board must fail (a known-bad board); empty for a good one.",
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)
