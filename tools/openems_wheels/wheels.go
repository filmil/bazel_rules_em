// SPDX-License-Identifier: Apache-2.0

// Package wheels holds what the CSXCAD and openEMS wheels are built from:
// the container image and the sources, each pinned, and the podman command
// that builds them (build.sh, embedded).
package wheels

import (
	_ "embed"
	"fmt"
	"sort"
)

// Image is the manylinux_2_28 build image (AlmaLinux 8, glibc 2.28), the
// base of upstream openEMS's own CI, pinned by digest.
const Image = "quay.io/pypa/manylinux_2_28_x86_64@sha256:43f7ce12d79ed98b6dbfa3291d5afe389efd40a7e76de089959ffe99501b6edf"

// Source is one upstream source tarball.
type Source struct {
	Name    string // FPARSER, CSXCAD or OPENEMS
	URL     string
	SHA256  string
	// Version is the Python package's version, "" when there is none: the
	// upstream release it follows, ".post", and the commit's date. A local
	// version ("+g626b99f") would put a "+" in the wheel's name, which a
	// GitHub release asset name does not keep.
	Version string
}

// Sources are the pinned upstream commits. openEMS d686f1d and CSXCAD
// 626b99f are the first commits that need no VTK, CGAL or Boost beyond five
// components (https://github.com/filmil/bazel_rules_em/issues/1).
var Sources = []Source{
	{
		Name:   "FPARSER",
		URL:    "https://github.com/thliebig/fparser/archive/4b9c845b449b520c4b8c5f23c74cd04820084f81.tar.gz",
		SHA256: "07f320b67df0f80a13766b0a74b2372edcba465ee33366bc57962bdfb76c0d1b",
	},
	{
		Name:    "CSXCAD",
		URL:     "https://github.com/thliebig/CSXCAD/archive/626b99f2f4d681cb012516b6522b7bc7068ce39c.tar.gz",
		SHA256:  "5b31a23846df1a8c7abcb6cf11c2957c14fb29dc9d1cdb01fd8549931eab82e7",
		Version: "0.7.0.post20261021",
	},
	{
		Name:    "OPENEMS",
		URL:     "https://github.com/thliebig/openEMS/archive/d686f1dfbb8900a748eacf1925e091ee32d1995c.tar.gz",
		SHA256:  "e42037d8fae0fc6f36072fb8dfff98c17f0d37c7023c7d107ab144c0f9e9672b",
		Version: "0.37.0.post20261024",
	},
}

//go:embed build.sh
var BuildScript string

// Command returns the podman command line that builds the wheels into out
// (an absolute directory) with jobs parallel compile jobs.
func Command(out string, jobs int) []string {
	args := []string{"podman", "run", "--rm", "-i",
		fmt.Sprintf("--cpus=%d", jobs),
		"-v", out + ":/out:Z",
		"-e", fmt.Sprintf("JOBS=%d", jobs),
	}
	var env []string
	for _, s := range Sources {
		env = append(env, "SRC_"+s.Name+"_URL="+s.URL, "SRC_"+s.Name+"_SHA256="+s.SHA256)
		if s.Version != "" {
			env = append(env, "SRC_"+s.Name+"_VERSION="+s.Version)
		}
	}
	sort.Strings(env)
	for _, e := range env {
		args = append(args, "-e", e)
	}
	// The script comes on standard input: bash -s reads it from there.
	return append(args, Image, "bash", "-s")
}
