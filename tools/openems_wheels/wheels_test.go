// SPDX-License-Identifier: Apache-2.0

package wheels

import (
	"regexp"
	"strings"
	"testing"
)

// Every source is pinned to a commit and a sha256, and the script checks
// the sha256 before it unpacks anything.
func TestSourcesArePinned(t *testing.T) {
	commit := regexp.MustCompile(`/archive/[0-9a-f]{40}\.tar\.gz$`)
	sha := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, s := range Sources {
		if !commit.MatchString(s.URL) {
			t.Errorf("%s: URL %q is not a commit tarball", s.Name, s.URL)
		}
		if !sha.MatchString(s.SHA256) {
			t.Errorf("%s: sha256 %q", s.Name, s.SHA256)
		}
	}
	if !strings.Contains(BuildScript, "sha256sum -c -") {
		t.Error("build.sh does not check the sources' sha256")
	}
}

// The image is pinned by digest, and the command passes every source.
func TestCommand(t *testing.T) {
	cmd := strings.Join(Command("/o", 3), " ")
	for _, want := range []string{
		"@sha256:", "-v /o:/out:Z", "JOBS=3", "--cpus=3",
		"SRC_OPENEMS_SHA256=e42037d8", "SRC_CSXCAD_VERSION=0.7.0.post20261021",
		"bash -s",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command lacks %q:\n%s", want, cmd)
		}
	}
	if !strings.HasPrefix(cmd, "podman run --rm -i ") {
		t.Errorf("command: %s", cmd)
	}
}

// The Python build tools and the C++ libraries are pinned to versions.
func TestScriptPinsVersions(t *testing.T) {
	for _, want := range []string{"cmake==", "cython==", "setuptools==", "numpy==",
		"hdf5-devel-1.10.5", "boost-devel-1.66.0", "tinyxml-devel-2.6.2", "SOURCE_DATE_EPOCH="} {
		if !strings.Contains(BuildScript, want) {
			t.Errorf("build.sh lacks %q", want)
		}
	}
}
