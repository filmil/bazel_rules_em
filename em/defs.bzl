# SPDX-License-Identifier: Apache-2.0
"""Public API of hdlf_rules_em.

Load every rule from this file:

```starlark
load(
    "@hdlf_rules_em//em:defs.bzl",
    "determinism_test",
    "swap_test",
    "tool_toolchain",
)
```

This is the scaffold. The EM rules, `kicad_em_model`, `kicad_em_sim` and
`kicad_em_test`, arrive after the openEMS toolchain, one pull request
each, as https://github.com/filmil/bazel_rules_em/issues/1 lays out. This
file is the only public entry point; everything under `//em/private` may
change without notice.
"""

load("//em/private:determinism.bzl", _determinism_test = "determinism_test")
load("//em/private:swap_test.bzl", _swap_test = "swap_test")
load("//em/private:toolchains.bzl", _tool_toolchain = "tool_toolchain")

determinism_test = _determinism_test
swap_test = _swap_test
tool_toolchain = _tool_toolchain

# The release this file belongs to. It lets a consumer check, in a test,
# that the module it resolved is the one it expects.
RULES_EM_API_VERSION = "0"
