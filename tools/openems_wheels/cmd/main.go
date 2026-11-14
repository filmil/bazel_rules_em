// SPDX-License-Identifier: Apache-2.0

// Command openems_wheels builds the CSXCAD and openEMS wheels that the
// openEMS toolchain installs, in the pinned manylinux container. The
// operator runs it by hand, with podman on the host:
//
//	bazel run //tools:openems_wheels -- --out=/tmp/oe-wheels
//
// A relative --out is taken from the directory bazel run was started in.
// It is not part of any build: its output is checked against
// tools/mirror/manifest.json and uploaded as a mirror release. With
// --dry_run it prints the podman command and the script instead.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/filmil/bazel_rules_em/tools/openems_wheels"
)

func main() {
	out := flag.String("out", "", "directory for the wheels, the sources, the licences and SHA256SUMS (required)")
	jobs := flag.Int("jobs", 4, "parallel compile jobs, and the container's CPUs")
	dry := flag.Bool("dry_run", false, "print the command and the script; run nothing")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "openems_wheels: --out is required")
		os.Exit(2)
	}
	if *jobs < 1 {
		fmt.Fprintln(os.Stderr, "openems_wheels: --jobs must be 1 or more")
		os.Exit(2)
	}
	// bazel run starts the tool in its runfiles; a relative --out means the
	// directory bazel run was started in.
	abs := *out
	if !filepath.IsAbs(abs) {
		wd := os.Getenv("BUILD_WORKING_DIRECTORY")
		if wd == "" {
			fmt.Fprintf(os.Stderr, "openems_wheels: --out %q is relative and BUILD_WORKING_DIRECTORY is not set; give an absolute path\n", abs)
			os.Exit(2)
		}
		abs = filepath.Join(wd, abs)
	}
	cmd := wheels.Command(abs, *jobs)
	if *dry {
		fmt.Println(strings.Join(cmd, " "))
		fmt.Print(wheels.BuildScript)
		return
	}
	// An old wheel left in --out would end up in SHA256SUMS.
	if entries, err := os.ReadDir(abs); err == nil && len(entries) > 0 {
		fmt.Fprintf(os.Stderr, "openems_wheels: --out %s is not empty\n", abs)
		os.Exit(2)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "openems_wheels:", err)
		os.Exit(2)
	}
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdin = strings.NewReader(wheels.BuildScript)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "openems_wheels:", err)
		os.Exit(1)
	}
}
