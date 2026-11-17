// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLine writes a Touchstone file of a matched, lossless 1 ns line.
func writeLine(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("# Hz S RI R 50\n")
	for k := 1; k <= 200; k++ {
		f := float64(k) * 25e6
		ph := -2 * math.Pi * f * 1e-9
		fmt.Fprintf(&b, "%g 0 0 %.9e %.9e %.9e %.9e 0 0\n", f, math.Cos(ph), math.Sin(ph), math.Cos(ph), math.Sin(ph))
	}
	p := filepath.Join(dir, "line.s2p")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func setup(t *testing.T, limits string, expect string) string {
	t.Helper()
	dir := t.TempDir()
	lp := filepath.Join(dir, "limits.json")
	if err := os.WriteFile(lp, []byte(limits), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	os.MkdirAll(out, 0o755)
	t.Setenv("EMCHECK_TOUCHSTONE", writeLine(t, dir))
	t.Setenv("EMCHECK_PORTS", "P1,P2")
	t.Setenv("EMCHECK_LIMITS", lp)
	t.Setenv("EMCHECK_EXPECT_FAIL", expect)
	t.Setenv("TEST_UNDECLARED_OUTPUTS_DIR", out)
	return out
}

const limits = `{"checks": [
 {"name": "loss", "kind": "insertion_loss", "from": "P1", "to": "P2", "max_db": 0.5},
 {"name": "return", "kind": "return_loss", "port": "P1", "min_db": 20}
]}`

func TestRun(t *testing.T) {
	// A matched, lossless line passes both checks.
	out := setup(t, limits, "")
	if err := run(); err != nil {
		t.Errorf("a good line: %v", err)
	}
	// A must-fail check that passes fails the run.
	setup(t, limits, "loss")
	if err := run(); err == nil || !strings.Contains(err.Error(), "loss passed") {
		t.Errorf("a must-fail check that passes: %v", err)
	}
	md, err := os.ReadFile(filepath.Join(out, "report.md"))
	if err != nil || !strings.Contains(string(md), "| loss | insertion_loss |") {
		t.Errorf("report.md: %q, %v", md, err)
	}
	setup(t, `{"checks": [{"name": "rl", "kind": "return_loss", "port": "P1", "min_db": 400}]}`, "")
	if err := run(); err == nil || !strings.Contains(err.Error(), "rl failed") {
		t.Errorf("a violated limit: %v", err)
	}
	setup(t, limits, "")
	t.Setenv("EMCHECK_LIMITS", "no/such/file.json")
	if err := run(); err == nil {
		t.Error("a missing limits file: no error")
	}
}
