# SPDX-License-Identifier: Apache-2.0
"""The openEMS runner: runs a simulation script with a fixed engine and
thread count, and makes its output files reproducible.

    runner --script SIM.py --out DIR [--threads N] [--params JSON]

SIM.py defines `simulate(out_dir, params, run)`. It builds its model with
CSXCAD and openEMS, and calls `run(fdtd, sim_path)` to solve it, never
`fdtd.Run` itself: `run` passes `engine="multithreaded"` and
`numThreads=N`, so the result does not depend on the machine (openEMS's
defaults pick the fastest engine and use every core). Everything the
script writes goes under out_dir.

openEMS writes the wall-clock time into the first line of each probe file
(`% time-domain ... by openEMS d686f1d @Sun Nov  1 09:38:27 2026`).
After the script returns, the runner removes that time from the first
line of every file under out_dir where it is; nothing else is read or
changed, so binary outputs such as HDF5 field dumps stay as they are.
"""

import argparse
import importlib.util
import json
import os
import re
import sys

ENGINE = "multithreaded"

# "% <anything> by openEMS <version> @<ctime>": a probe file's first line.
_STAMP = re.compile(rb"^(%[^\n]* by openEMS \S+) @[^\n]*$")


class RunnerError(Exception):
    pass


def strip_times(data):
    """data with the openEMS time stamp removed from its first line."""
    first, nl, rest = data.partition(b"\n")
    return _STAMP.sub(rb"\1", first) + nl + rest


def normalize_tree(root):
    """Strips the time stamps from every file under root, in place."""
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames.sort()
        for name in sorted(filenames):
            path = os.path.join(dirpath, name)
            with open(path, "rb") as f:
                first = f.readline(4096)
            if not first.startswith(b"%") or not _STAMP.match(first.rstrip(b"\n")):
                continue
            with open(path, "rb") as f:
                data = f.read()
            with open(path, "wb") as f:
                f.write(strip_times(data))


def make_run(threads):
    """The run(fdtd, sim_path) a script calls to solve its model."""

    def run(fdtd, sim_path):
        fdtd.Run(sim_path, cleanup=True, engine=ENGINE, numThreads=threads, verbose=0)

    return run


def load_script(path):
    if not os.path.isfile(path):
        raise RunnerError("%s: no such file" % path)
    spec = importlib.util.spec_from_file_location("em_simulation", path)
    if spec is None or spec.loader is None:
        raise RunnerError("%s: not a Python file" % path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    if not callable(getattr(mod, "simulate", None)):
        raise RunnerError("%s: no simulate(out_dir, params, run) function" % path)
    return mod


def main(argv):
    p = argparse.ArgumentParser(prog="runner")
    p.add_argument("--script", required=True, help="the simulation script")
    p.add_argument("--out", required=True, help="the output directory")
    p.add_argument("--threads", type=int, default=2, help="openEMS's thread count, fixed")
    p.add_argument("--params", default="{}", help="the parameters, as a JSON object")
    a = p.parse_args(argv)
    if a.threads < 1:
        p.error("--threads must be 1 or more")
    # openEMS's Run changes the working directory to its sim_path, so every
    # path is made absolute first, and the directory is restored after.
    out = os.path.abspath(a.out)
    script = os.path.abspath(a.script)
    cwd = os.getcwd()
    os.makedirs(out, exist_ok=True)
    try:
        params = json.loads(a.params)
        if not isinstance(params, dict):
            raise RunnerError("--params must be a JSON object, not %s" % type(params).__name__)
        load_script(script).simulate(out, params, make_run(a.threads))
    except (RunnerError, ValueError) as e:
        print("runner: %s" % e, file=sys.stderr)
        return 1
    finally:
        os.chdir(cwd)
    normalize_tree(out)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
