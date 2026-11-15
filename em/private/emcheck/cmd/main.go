// SPDX-License-Identifier: Apache-2.0

// Command emcheck is kicad_em_test's test: it checks a kicad_em_sim's
// S-parameters against a limits file. It reads, from the environment:
//
//   - EMCHECK_TOUCHSTONE: the Touchstone file, a runfiles location;
//   - EMCHECK_PORTS: the ports' names, comma-separated, in its order;
//   - EMCHECK_LIMITS: the limits file, a runfiles location;
//   - EMCHECK_EXPECT_FAIL: the checks this board must fail, comma-separated
//     (a known-bad board), or empty.
//
// It prints the results and writes them to report.md in the test's
// undeclared outputs.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/filmil/bazel_rules_em/em/private/emcheck"
)

// path resolves a runfiles location; a path that exists as given (an
// absolute one, in tests) is used as it is.
func path(loc string) (string, error) {
	if _, err := os.Stat(loc); err == nil {
		return loc, nil
	}
	return runfiles.Rlocation(loc)
}

func list(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func run() error {
	ts, err := path(os.Getenv("EMCHECK_TOUCHSTONE"))
	if err != nil {
		return err
	}
	data, err := os.ReadFile(ts)
	if err != nil {
		return err
	}
	nw, err := emcheck.ParseTouchstone(ts, data)
	if err != nil {
		return err
	}
	lp, err := path(os.Getenv("EMCHECK_LIMITS"))
	if err != nil {
		return err
	}
	ld, err := os.ReadFile(lp)
	if err != nil {
		return err
	}
	limits, err := emcheck.ParseLimits(ld)
	if err != nil {
		return fmt.Errorf("%s: %v", os.Getenv("EMCHECK_LIMITS"), err)
	}
	expect := list(os.Getenv("EMCHECK_EXPECT_FAIL"))
	results, err := emcheck.Run(nw, list(os.Getenv("EMCHECK_PORTS")), limits)
	if err != nil {
		return err
	}
	report := emcheck.Report(filepath.Base(ts), results, expect)
	fmt.Print(report)
	if d := os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"); d != "" {
		if err := os.WriteFile(filepath.Join(d, "report.md"), []byte(report), 0o644); err != nil {
			return err
		}
	}
	return emcheck.Verdict(results, expect)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "kicad_em_test:", err)
		os.Exit(1)
	}
}
