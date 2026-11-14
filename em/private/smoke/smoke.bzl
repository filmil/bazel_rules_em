# SPDX-License-Identifier: Apache-2.0
"""openems_run: runs one simulation script on the openEMS toolchain.

It is the toolchain's smoke test (issue #1), and private: the EM rules
(kicad_em_model, kicad_em_sim) arrive in later pull requests and run the
runner the same way.
"""

load("//em/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

TOOLCHAIN_TYPE = Label("//em:toolchain_type")

# numpy's BLAS reads these; one thread keeps its sums in a fixed order.
_ONE_THREAD = {
    "MKL_NUM_THREADS": "1",
    "OMP_NUM_THREADS": "1",
    "OPENBLAS_NUM_THREADS": "1",
}

def _openems_run_impl(ctx):
    out = ctx.actions.declare_directory(ctx.label.name)
    run_tool(
        ctx,
        TOOLCHAIN_TYPE,
        "openems",
        arguments = [
            "--script",
            ctx.file.script.path,
            "--out",
            out.path,
            "--threads",
            str(ctx.attr.threads),
        ],
        inputs = [ctx.file.script],
        outputs = [out],
        mnemonic = "OpenEMS",
        progress_message = "Solving %{label} with openEMS",
        env = _ONE_THREAD,
        # matplotlib, which CSXCAD imports, writes a cache there.
        home_env = ["MPLCONFIGDIR"],
    )
    return [DefaultInfo(files = depset([out]))]

openems_run = rule(
    implementation = _openems_run_impl,
    doc = "Runs a simulation script (simulate(out_dir, params, run)) with the openEMS runner; the output is a directory.",
    attrs = {
        "script": attr.label(allow_single_file = [".py"], mandatory = True),
        "threads": attr.int(default = 2, doc = "openEMS's thread count, fixed."),
    },
    toolchains = [TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
