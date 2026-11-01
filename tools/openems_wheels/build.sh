#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
#
# Builds the CSXCAD and openEMS wheels for CPython 3.12, manylinux_2_28.
# It runs inside the pinned manylinux container that
# `bazel run //tools:openems_wheels` starts, never on the host, with the
# output directory mounted at /out. The Go tool embeds this file and passes
# the pinned sources as environment variables:
#
#   SRC_<NAME>_URL, SRC_<NAME>_SHA256, SRC_<NAME>_VERSION for NAME in
#   FPARSER, CSXCAD, OPENEMS.
#
# /out receives the two wheels, the three source tarballs as downloaded,
# the licences of everything in the wheels, and SHA256SUMS.
set -euo pipefail

# Fixed times in the wheels' zip entries, so two builds give the same bytes.
export SOURCE_DATE_EPOCH=315532800
export PIP_DISABLE_PIP_VERSION_CHECK=1
JOBS=${JOBS:-4}
PY=/opt/python/cp312-cp312/bin/python

# The C++ dependencies, at the versions the wheels were first built with.
dnf install -y -q epel-release
dnf install -y -q \
  hdf5-devel-1.10.5-4.el8 \
  tinyxml-devel-2.6.2-28.el8 \
  boost-devel-1.66.0-13.el8 \
  zlib-devel

# The Python build tools, pinned.
$PY -m pip install -q \
  cmake==4.4.4 cython==3.3.0 setuptools==84.2.1 setuptools-scm==10.3.4 \
  numpy==2.5.4 wheel==0.48.0
CMAKE=$($PY -c 'import cmake, os; print(os.path.join(cmake.CMAKE_BIN_DIR, "cmake"))')

# Each source: downloaded, checked against its sha256, unpacked.
fetch() {
  local name=$1 url_var=SRC_${1}_URL sha_var=SRC_${1}_SHA256
  local tgz=/out/${name,,}-src.tar.gz
  curl -fsSL -o "$tgz" "${!url_var}"
  echo "${!sha_var}  $tgz" | sha256sum -c -
  mkdir -p "/src/$name"
  tar -xzf "$tgz" -C "/src/$name" --strip-components=1
}
for n in FPARSER CSXCAD OPENEMS; do fetch "$n"; done

P=/opt/oe
for n in FPARSER CSXCAD OPENEMS; do
  "$CMAKE" -S "/src/$n" -B "/tmp/build-$n" -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX=$P -DCMAKE_PREFIX_PATH=$P \
    -DFPARSER_ROOT_DIR=$P -DCSXCAD_ROOT_DIR=$P
  "$CMAKE" --build "/tmp/build-$n" -j"$JOBS"
  "$CMAKE" --install "/tmp/build-$n"
done

# The Python bindings. Without git, setuptools_scm takes the version from
# SETUPTOOLS_SCM_PRETEND_VERSION. openEMS's binding cimports CSXCAD's, so
# CSXCAD is installed before openEMS is built.
export CSXCAD_INSTALL_PATH=$P OPENEMS_INSTALL_PATH=$P LD_LIBRARY_PATH=$P/lib:$P/lib64
mkdir -p /tmp/raw
SETUPTOOLS_SCM_PRETEND_VERSION=$SRC_CSXCAD_VERSION \
  $PY -m pip wheel -q --no-build-isolation --no-deps -w /tmp/raw /src/CSXCAD/python
$PY -m pip install -q /tmp/raw/csxcad-*.whl
SETUPTOOLS_SCM_PRETEND_VERSION=$SRC_OPENEMS_VERSION \
  $PY -m pip wheel -q --no-build-isolation --no-deps -w /tmp/raw /src/OPENEMS/python

# auditwheel copies the shared libraries each wheel needs into it.
for w in /tmp/raw/*.whl; do
  auditwheel repair --plat manylinux_2_28_x86_64 -w /out "$w"
done

# The licences of what the wheels hold. Each file must exist: a missing
# licence stops the build.
L=/out/licenses
mkdir -p $L
lic() {
  local dst=$1; shift
  local f
  for f in "$@"; do
    if [ -f "$f" ]; then cp "$f" "$L/$dst"; return; fi
  done
  echo "no licence file for $dst in: $*" >&2
  exit 1
}
lic openEMS-GPL-3.0.txt /src/OPENEMS/COPYING
lic CSXCAD-LGPL-3.0.txt /src/CSXCAD/COPYING
lic fparser-LGPL-3.0.txt /src/FPARSER/docs/lgpl.txt
lic hdf5.txt /usr/share/licenses/hdf5/COPYING
lic boost-BSL-1.0.txt /usr/share/licenses/boost-system/LICENSE_1_0.txt
lic libaec.txt /usr/share/licenses/libaec/Copyright.txt
lic libaec-patent.txt /usr/share/licenses/libaec/patent.txt
# TinyXML ships no licence file, and the image installs no docs. Its
# header's first comment is the zlib licence.
sed -n '1,/\*\//p' /usr/include/tinyxml.h > $L/tinyxml-Zlib.txt
grep -q 'This notice may not be removed or altered' $L/tinyxml-Zlib.txt || {
  echo "tinyxml.h does not start with its licence" >&2; exit 1; }
lic zlib.txt /usr/share/licenses/zlib/README
rpm -q --qf '%{NAME} %{VERSION}-%{RELEASE} %{LICENSE}\n' hdf5 libaec tinyxml boost-system zlib > $L/PACKAGES.txt
tar --sort=name --mtime=@$SOURCE_DATE_EPOCH --owner=0 --group=0 --numeric-owner \
  -C /out -cf /out/openems-wheels-licenses.tar licenses
rm -rf $L

cd /out
sha256sum -- *.whl *.tar.gz *.tar > SHA256SUMS
cat SHA256SUMS
