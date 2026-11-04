<!-- SPDX-License-Identifier: Apache-2.0 -->
[![Test](https://github.com/filmil/bazel_rules_em/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/filmil/bazel_rules_em/actions/workflows/test.yml)
[![Tag and Release](https://github.com/filmil/bazel_rules_em/actions/workflows/tag-and-release.yml/badge.svg)](https://github.com/filmil/bazel_rules_em/releases)
[![Publish to my Bazel registry](https://github.com/filmil/bazel_rules_em/actions/workflows/publish.yml/badge.svg)](https://github.com/filmil/bazel_rules_em/actions/workflows/publish.yml)
[![Publish on Bazel Central Registry](https://github.com/filmil/bazel_rules_em/actions/workflows/publish-bcr.yml/badge.svg)](https://github.com/filmil/bazel_rules_em/actions/workflows/publish-bcr.yml)

# hdlf_rules_em

Hermetic Bazel rules for the electromagnetic analysis of KiCad boards with
[openEMS](https://openems.de), an FDTD solver.
The build will turn a board's `.kicad_pcb` and its stackup into an openEMS
model, solve it, and give Touchstone S-parameter files and impedance and
TDR plots. A test will check them against limits per net or pair, such as
90 Ω ±15 % differential impedance for a USB pair.
Every tool comes from a Bazel toolchain, so nothing needs to be installed
on the machine, and two builds give byte-identical outputs.

It is part of the hdlf hardware rules, next to
[hdlf_rules_kicad](https://github.com/filmil/bazel_rules_kicad) and
[hdlf_rules_fab](https://github.com/filmil/bazel_rules_fab).

Only Linux x86_64 hosts are supported.

## Status

This is the scaffold: the module, its helpers and tests, and its CI and
release workflows. The openEMS toolchain is next. After it come
`kicad_em_model`, `kicad_em_sim` and `kicad_em_test`, one pull request each;
https://github.com/filmil/bazel_rules_em/issues/1 has the plan.

What works now are the test rules every hdlf module shares:

* `determinism_test` builds a target twice and fails if any output
  differs;
* `swap_test` builds targets with every tool swapped for a stub, and fails
  if any output was made without a toolchain tool.

## Using it

There is no release yet; 0.0.1, below, will be the first.

`MODULE.bazel`:

```starlark
bazel_dep(name = "hdlf_rules_em", version = "0.0.1")
```

`.bazelrc`, while the module is only in the filmil registry:

```
common --registry=https://raw.githubusercontent.com/filmil/bazel-registry/main
common --registry=https://bcr.bazel.build
```

`BUILD.bazel`:

```starlark
load("@hdlf_rules_em//em:defs.bzl", "determinism_test", "swap_test")
```

Every rule is in [docs/defs.md](docs/defs.md).

## Building

```sh
bazel build //...
bazel test //...
cd integration && bazel test //...
```

After changing a `.bzl` file, regenerate the documentation with
`bazel run //docs:update_docs`.
Format with `bazel run //tools:buildifier`, and check hermeticity with
`bazel run //tools:lint`.

## Releasing

The operator runs the `Tag and Release` workflow
(`.github/workflows/tag-and-release.yml`, `workflow_dispatch`).
It picks the next version from the conventional commits since the last
tag, makes the tag, the release and its source archive, and opens one
pull request on the filmil registry and one on the Bazel Central Registry.
It needs the repository secret `BCR_PUBLISH_TOKEN`.

None of the modules published here are published with any sort of guarantee
whatsoever. Use at your own risk.

See the overall description at the [project page][pp].

[pp]: https://hdlfactory.com/post/2025/09/29/getting-ready-for-the-brave-new-bazel-modules-world/

## Licenses

This repository is under the Apache License 2.0 (`LICENSE`).
The tools and libraries that the build downloads are listed here, with their
versions, sources and licenses.

| What | Version | Source | License |
|---|---|---|---|
| Go SDK (builds the helpers: the launcher, the swap test's stub and checker, the determinism comparer) | as rules_go 0.64.2 picks it | [go.dev/dl](https://go.dev/dl/), fetched by rules_go | BSD-3-Clause |
