// SPDX-License-Identifier: Apache-2.0

// Package sim_test reads kicad_em_sim's outputs for the fixture's D+ line,
// with tolerances: FDTD's numbers are not compared bit for bit.
package sim_test

import (
	"bufio"
	"flag"
	"image/png"
	"math"
	"math/cmplx"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var dir = flag.String("dir", "", "kicad_em_sim's output directory, as a runfiles path")

func out(t *testing.T) string {
	t.Helper()
	p, err := runfiles.Rlocation(*dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// readS2P reads a Touchstone 1.1 two-port file in Hz, S, RI.
func readS2P(t *testing.T, path string) (freqs []float64, s [][4]complex128) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if line != "# Hz S RI R 50" {
				t.Fatalf("option line %q", line)
			}
			continue
		}
		var v []float64
		for _, x := range strings.Fields(line) {
			n, err := strconv.ParseFloat(x, 64)
			if err != nil {
				t.Fatalf("%q: %v", line, err)
			}
			v = append(v, n)
		}
		if len(v) != 9 {
			t.Fatalf("%q: %d numbers, want 9", line, len(v))
		}
		freqs = append(freqs, v[0])
		s = append(s, [4]complex128{
			complex(v[1], v[2]), complex(v[3], v[4]), complex(v[5], v[6]), complex(v[7], v[8]),
		})
	}
	return freqs, s
}

func db(c complex128) float64 { return 20 * math.Log10(cmplx.Abs(c)) }

func TestTouchstone(t *testing.T) {
	freqs, s := readS2P(t, filepath.Join(out(t), "line_sim.s2p"))
	if len(freqs) != 21 || freqs[0] != 1e8 || freqs[20] != 6e9 {
		t.Fatalf("frequencies %v", freqs)
	}
	// At 100 MHz the line is short against the wavelength: it passes.
	if s21 := db(s[0][1]); s21 < -1 {
		t.Errorf("S21 at 100 MHz is %.2f dB, want above -1 dB", s21)
	}
	if s11 := db(s[0][0]); s11 > -10 {
		t.Errorf("S11 at 100 MHz is %.2f dB, want below -10 dB", s11)
	}
	for k, f := range freqs {
		// Passive and reciprocal.
		for i, c := range s[k] {
			if cmplx.Abs(c) > 1.05 {
				t.Errorf("|S[%d]| at %g Hz is %.3f, above 1", i, f, cmplx.Abs(c))
			}
		}
		if d := cmplx.Abs(s[k][1] - s[k][2]); d > 0.05 {
			t.Errorf("S21 and S12 at %g Hz differ by %.3f", f, d)
		}
	}
}

func TestPlotsAndReport(t *testing.T) {
	d := out(t)
	for _, name := range []string{"s_params.png", "impedance.png", "tdr.png"} {
		f, err := os.Open(filepath.Join(d, name))
		if err != nil {
			t.Error(err)
			continue
		}
		if _, err := png.Decode(f); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		f.Close()
	}
	md, err := os.ReadFile(filepath.Join(d, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# line_sim", "2 ports", "| P1 | J1.1 | D+ | F.Cu |", "| P2 | R1.2 | D+ | F.Cu |", "## TDR impedance"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("report.md has no %q:\n%s", want, md)
		}
	}
}
