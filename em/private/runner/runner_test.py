# SPDX-License-Identifier: Apache-2.0
"""Tests of the runner's parts that need no solver."""

import os
import tempfile
import unittest

import runner


class StripTimesTest(unittest.TestCase):
    def test_header_time_removed(self):
        data = (b"% time-domain voltage integration by openEMS d686f1d @Sun Nov  1 09:38:27 2026\n"
                b"% t/s\tvoltage\n0\t1.5\n")
        want = (b"% time-domain voltage integration by openEMS d686f1d\n"
                b"% t/s\tvoltage\n0\t1.5\n")
        self.assertEqual(runner.strip_times(data), want)

    def test_data_untouched(self):
        data = b"% no stamp here\n1 @ 2\nby openEMS x @y\n"
        self.assertEqual(runner.strip_times(data), data)

    def test_tree(self):
        with tempfile.TemporaryDirectory() as d:
            os.makedirs(os.path.join(d, "sub"))
            p = os.path.join(d, "sub", "port_ut_1")
            with open(p, "wb") as f:
                f.write(b"% time-domain current integration by openEMS v1 @Mon Jan  1 00:00:00 2026\n1\t2\n")
            runner.normalize_tree(d)
            with open(p, "rb") as f:
                self.assertEqual(f.read(), b"% time-domain current integration by openEMS v1\n1\t2\n")


class ScriptTest(unittest.TestCase):
    def test_no_simulate(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "sim.py")
            with open(p, "w") as f:
                f.write("x = 1\n")
            with self.assertRaises(runner.RunnerError):
                runner.load_script(p)

    def test_missing_script(self):
        with self.assertRaises(runner.RunnerError):
            runner.load_script("/no/such/sim.py")

    def test_run_fixes_engine_and_threads(self):
        calls = []

        class FakeFDTD:
            def Run(self, sim_path, **kw):
                calls.append((sim_path, kw))

        runner.make_run(3)(FakeFDTD(), "/x")
        self.assertEqual(calls, [("/x", {"cleanup": True, "engine": "multithreaded", "numThreads": 3, "verbose": 0})])


def _run_main(script, *args):
    """Writes script as sim.py in a new directory, runs main there, and
    returns (exit code, directory, working directory after)."""
    d = tempfile.mkdtemp()
    old = os.getcwd()
    os.chdir(d)
    try:
        with open("sim.py", "w") as f:
            f.write(script)
        code = runner.main(["--script", "sim.py", "--out", "out"] + list(args))
        return code, d, os.getcwd()
    finally:
        os.chdir(old)


class MainTest(unittest.TestCase):
    def test_params_reach_simulate(self):
        script = (
            "import json, os\n"
            "def simulate(out_dir, params, run):\n"
            "    open(os.path.join(out_dir, 'p.json'), 'w').write(json.dumps(params, sort_keys=True))\n"
        )
        code, d, _ = _run_main(script, "--params", '{"b": 2, "a": 1}')
        self.assertEqual(code, 0)
        with open(os.path.join(d, "out", "p.json")) as f:
            self.assertEqual(f.read(), '{"a": 1, "b": 2}')

    def test_bad_params(self):
        script = "def simulate(out_dir, params, run):\n    pass\n"
        self.assertEqual(_run_main(script, "--params", "{not json")[0], 1)
        self.assertEqual(_run_main(script, "--params", "[1, 2]")[0], 1)

    def test_threads_must_be_positive(self):
        with self.assertRaises(SystemExit):
            _run_main("def simulate(out_dir, params, run):\n    pass\n", "--threads", "0")

    def test_cwd_restored_when_simulate_raises(self):
        script = (
            "import os\n"
            "def simulate(out_dir, params, run):\n"
            "    os.chdir(out_dir)\n"
            "    raise RuntimeError('solver failed')\n"
        )
        d = tempfile.mkdtemp()
        old = os.getcwd()
        os.chdir(d)
        try:
            with open("sim.py", "w") as f:
                f.write(script)
            with self.assertRaises(RuntimeError):
                runner.main(["--script", "sim.py", "--out", "out"])
            self.assertEqual(os.getcwd(), os.path.realpath(d))
        finally:
            os.chdir(old)

    def test_binary_file_untouched(self):
        with tempfile.TemporaryDirectory() as d:
            data = b"\x89HDF\r\n\x1a\n% x by openEMS v1 @Mon Jan  1 00:00:00 2026\n"
            p = os.path.join(d, "e_field.h5")
            with open(p, "wb") as f:
                f.write(data)
            runner.normalize_tree(d)
            with open(p, "rb") as f:
                self.assertEqual(f.read(), data)


class MainChdirTest(unittest.TestCase):
    def test_stamps_removed_after_a_chdir(self):
        # openEMS's Run changes the working directory into sim_path; the
        # runner must still find and clean the files under a relative --out.
        script = (
            "import os\n"
            "def simulate(out_dir, params, run):\n"
            "    os.chdir(out_dir)\n"
            "    with open('port_ut_1', 'wb') as f:\n"
            "        f.write(b'% time-domain voltage integration by openEMS v0.37.0 @Sun Nov  1 12:04:09 2026\\n1\\t2\\n')\n"
        )
        with tempfile.TemporaryDirectory() as d:
            old = os.getcwd()
            os.chdir(d)
            try:
                with open("sim.py", "w") as f:
                    f.write(script)
                self.assertEqual(runner.main(["--script", "sim.py", "--out", "out"]), 0)
                self.assertEqual(os.getcwd(), os.path.realpath(d))
                with open(os.path.join(d, "out", "port_ut_1"), "rb") as f:
                    self.assertEqual(f.read(), b"% time-domain voltage integration by openEMS v0.37.0\n1\t2\n")
            finally:
                os.chdir(old)


if __name__ == "__main__":
    unittest.main()
