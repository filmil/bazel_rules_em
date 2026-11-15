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

The openEMS toolchain, `kicad_em_model` and `kicad_em_sim` are here. Next
comes `kicad_em_test`; https://github.com/filmil/bazel_rules_em/issues/1 has
the plan.

What works now:

* **`kicad_em_model`**: a board's nets as an openEMS model, read from the
  `.kicad_pcb` itself so that nets and ports keep the board's names. It
  writes the model as JSON, as CSXCAD geometry (`model.xml`) and as a
  picture, and runs no solver.
* **`kicad_em_sim`**: solves a model with openEMS, each port excited in
  turn, and writes the S-parameters (Touchstone), plots of |S|, input
  impedance and TDR, and a report. Each run takes a fixed number of time
  steps, so the same model gives the same bytes.
* **The openEMS toolchain** (`@hdlf_rules_em//em:toolchain_type`), with one
  tool, `openems`: a runner that solves a simulation script with openEMS
  d686f1d and CSXCAD 626b99f on CPython 3.12. It always runs openEMS's
  multithreaded engine with a fixed thread count, and removes the time
  stamps openEMS writes into its output, so two runs give the same bytes.
  The EM rules will use it; there is no public rule for it yet.
* `determinism_test` builds a target twice and fails if any output
  differs;
* `swap_test` builds targets with every tool swapped for a stub, and fails
  if any output was made without a toolchain tool.

### The openEMS wheels

PyPI has no CSXCAD or openEMS, and no Linux distribution ships the
post-VTK commits. So their Python wheels are built once, by
`bazel run //tools:openems_wheels -- --out=DIR`, in a manylinux_2_28
container pinned by digest (with podman, on the operator's machine), and
mirrored as the release `mirror-openems-d686f1d` of this repository,
together with the three source tarballs they are built from and the
licences of everything inside them. `requirements_lock.txt` pins each wheel
by its sha256. The wheels need glibc 2.28 or later on the host, as the
Python interpreter does.

### Slow tests

A target that runs the solver, and every test that depends on one, is
tagged `fdtd`; a test whose own run solves is also `size = "large"`.
`bazel test //...` leaves those out; `bazel test --config=fdtd //...` runs
only them, and CI runs that in a job of its own.

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
load("@hdlf_rules_em//em:defs.bzl", "kicad_em_model", "kicad_em_sim")

# The USB pair of a two-layer 1.6 mm board, from the connector to the
# microcontroller.
kicad_em_model(
    name = "usb_model",
    pcb = ":pcb",
    nets = ["USB_DP", "USB_DM", "GND"],
    ports = {"P1": "J1.A6", "P2": "J1.A7", "P3": "U1.18", "P4": "U1.17"},
    dielectrics = ["1.51mm 4.5 0.02"],
)

# Its S-parameters, impedance and TDR, from 100 MHz to 6 GHz. It runs
# the solver: tag it fdtd.
kicad_em_sim(
    name = "usb_sim",
    model = ":usb_model",
    f_start = "100MHz",
    f_stop = "6GHz",
    tags = ["fdtd"],
)
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
The solver tests run with `bazel test --config=fdtd //...`.

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
| CPython (runs the openEMS runner) | 3.12, as rules_python 2.4.6 picks it | [python-build-standalone](https://github.com/astral-sh/python-build-standalone), fetched by rules_python | PSF-2.0 |
| openEMS (the solver, in the `openems` wheel) | commit d686f1d, as 0.37.0.post20261024 | [thliebig/openEMS](https://github.com/thliebig/openEMS); wheel built by `//tools:openems_wheels`, mirrored as `mirror-openems-d686f1d` | GPL-3.0-or-later |
| CSXCAD (the geometry, in the `csxcad` wheel, and inside `openems`) | commit 626b99f, as 0.7.0.post20261021 | [thliebig/CSXCAD](https://github.com/thliebig/CSXCAD); as above | LGPL-3.0-or-later |
| fparser (inside both wheels) | commit 4b9c845 | [thliebig/fparser](https://github.com/thliebig/fparser); as above | LGPL-3.0-or-later |
| HDF5, libaec, TinyXML, Boost (inside the wheels, from AlmaLinux 8 and EPEL) | 1.10.5, 1.0.2, 2.6.2, 1.66.0 | the manylinux_2_28 image's package repositories | BSD-3-Clause-style (HDF5), BSD-2-Clause, Zlib, BSL-1.0 |
| numpy, h5py, matplotlib and their dependencies | as `requirements_lock.txt` pins them | PyPI | BSD-3-Clause (numpy, h5py), matplotlib's PSF-based licence, and theirs |

The licence texts of what the two wheels hold are in
`openems-wheels-licenses.tar`, an asset of the mirror release, beside the
exact sources the wheels are built from.
