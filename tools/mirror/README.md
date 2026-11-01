<!-- SPDX-License-Identifier: Apache-2.0 -->
# Mirror

`manifest.json` lists the files this repository mirrors as release
assets, and where each comes from. `bazel run //tools:mirror` checks
every upstream file against it; the operator uploads with
`--dry_run=false --built_dir=DIR` (see `cmd/main.go`).

## openEMS and CSXCAD wheels

PyPI has no CSXCAD or openEMS, and no distribution ships the post-VTK
commits the toolchain pins (https://github.com/filmil/bazel_rules_em/issues/1).
So their wheels are built once and mirrored as the release
`mirror-openems-d686f1d`, with the three source tarballs they are built
from and `openems-wheels-licenses.tar`, the licences of everything inside
them (openEMS is GPL-3.0-or-later, CSXCAD and fparser LGPL-3.0-or-later).

To build them, with podman on the host, twice, and check the two builds
agree:

```sh
bazel run //tools:openems_wheels -- --out=/tmp/oe1
bazel run //tools:openems_wheels -- --out=/tmp/oe2
diff /tmp/oe1/SHA256SUMS /tmp/oe2/SHA256SUMS
bazel run //tools:mirror -- --built_dir=/tmp/oe1
```

The tool runs `tools/openems_wheels/build.sh` in the manylinux_2_28 image
pinned by digest: it downloads each source and checks its sha256, builds
fparser, CSXCAD and openEMS with CMake, builds the two Python bindings,
and lets auditwheel copy the shared libraries they need into the wheels.
`SOURCE_DATE_EPOCH` fixes the times in the wheels, so two builds give the
same bytes; the last command checks them against the manifest. The
operator then uploads with `--dry_run=false --built_dir=/tmp/oe1`.

`requirements.in` names the two wheels by their release URLs, and
`requirements_lock.txt` pins them by sha256. `bazel run
//tools:requirements.update` regenerates the lock once the release exists.
