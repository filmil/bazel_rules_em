# SPDX-License-Identifier: Apache-2.0
"""The runner removed every openEMS time stamp from the smoke run's output.

determinism_test cannot see a stamp when both runs fall in the same second,
so this checks for one directly.
"""

import os
import re
import sys
import unittest

STAMP = re.compile(rb"by openEMS \S+ @")


class NoStampsTest(unittest.TestCase):
    def test_no_stamps(self):
        root = os.path.join(os.environ["TEST_SRCDIR"], os.environ["TEST_WORKSPACE"], sys.argv[1])
        seen = 0
        for dirpath, _, filenames in os.walk(root):
            for name in filenames:
                seen += 1
                with open(os.path.join(dirpath, name), "rb") as f:
                    self.assertIsNone(STAMP.search(f.read()), os.path.join(dirpath, name))
        self.assertGreater(seen, 0, "no output files under %s" % root)


if __name__ == "__main__":
    unittest.main(argv=sys.argv[:1])
