// SPDX-License-Identifier: Apache-2.0

package emcheck

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"
	"testing"
)

// line is an ideal lossless line of impedance zl and delay tau between
// two z0 ports: its S11 and S21 at f.
func line(zl, tau, z0, f float64) (complex128, complex128) {
	bl := 2 * math.Pi * f * tau
	a := complex(math.Cos(bl), 0)
	b := complex(0, zl*math.Sin(bl))
	c := complex(0, math.Sin(bl)/zl)
	d := a
	den := a + b/complex(z0, 0) + c*complex(z0, 0) + d
	return (a + b/complex(z0, 0) - c*complex(z0, 0) - d) / den, 2 / den
}

func grid() []float64 {
	var f []float64
	for k := 1; k <= 1000; k++ {
		f = append(f, float64(k)*10e6)
	}
	return f
}

// pairNetwork is two uncoupled lines: P1 -> P2 positive, P3 -> P4
// negative, the negative extra seconds longer.
func pairNetwork(zl, tau, extra float64) *Network {
	nw := &Network{N: 4, Z0: 50, Freqs: grid()}
	for _, f := range nw.Freqs {
		s := make([][]complex128, 4)
		for i := range s {
			s[i] = make([]complex128, 4)
		}
		p11, p21 := line(zl, tau, 50, f)
		n11, n21 := line(zl, tau+extra, 50, f)
		s[0][0], s[1][1], s[1][0], s[0][1] = p11, p11, p21, p21
		s[2][2], s[3][3], s[3][2], s[2][3] = n11, n11, n21, n21
		nw.S = append(nw.S, s)
	}
	return nw
}

// touchstone writes a network as Touchstone 1.1, RI.
func touchstone(nw *Network) string {
	var b strings.Builder
	b.WriteString("! test\n# Hz S RI R 50\n")
	for k, f := range nw.Freqs {
		fmt.Fprintf(&b, "%g", f)
		for idx := 0; idx < nw.N*nw.N; idx++ {
			i, j := idx/nw.N, idx%nw.N
			if nw.N == 2 {
				i, j = idx%2, idx/2
			}
			c := nw.S[k][i][j]
			fmt.Fprintf(&b, " %.9e %.9e", real(c), imag(c))
			if nw.N > 2 && idx%4 == 3 {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestTouchstone(t *testing.T) {
	nw := pairNetwork(60, 100e-12, 10e-12)
	got, err := ParseTouchstone("x.s4p", []byte(touchstone(nw)))
	if err != nil {
		t.Fatal(err)
	}
	if got.N != 4 || len(got.Freqs) != 1000 || got.Z0 != 50 {
		t.Fatalf("parsed %d ports, %d frequencies, z0 %v", got.N, len(got.Freqs), got.Z0)
	}
	if d := cmplx.Abs(got.S[500][3][2] - nw.S[500][3][2]); d > 1e-8 {
		t.Errorf("S43 differs by %g", d)
	}
	// Two ports: S11 S21 S12 S22; MA and DB formats.
	two := "# GHz S MA R 75\n1 0.5 90 0.25 0 0.125 0 1 180\n"
	n2, err := ParseTouchstone("y.S2P", []byte(two))
	if err != nil {
		t.Fatal(err)
	}
	if n2.Freqs[0] != 1e9 || n2.Z0 != 75 || cmplx.Abs(n2.S[0][0][0]-complex(0, 0.5)) > 1e-12 ||
		n2.S[0][1][0] != 0.25 || n2.S[0][0][1] != 0.125 {
		t.Errorf("two-port %+v", n2)
	}
	n3, err := ParseTouchstone("z.s1p", []byte("# MHz S DB\n100 -20 0\n"))
	if err != nil || math.Abs(cmplx.Abs(n3.S[0][0][0])-0.1) > 1e-12 || n3.Freqs[0] != 1e8 {
		t.Errorf("DB one-port %+v, %v", n3, err)
	}
	for _, bad := range []struct{ name, text string }{
		{"x.txt", "# Hz S RI\n1 0 0\n"},
		{"x.s1p", "# Hz Y RI\n1 0 0\n"},
		{"x.s1p", "# Hz S RI\n1 0\n"},
		{"x.s1p", "# Hz S RI\n2 0 0\n1 0 0\n"},
		{"x.s1p", "# Hz S RI\n1 0 x\n"},
	} {
		if _, err := ParseTouchstone(bad.name, []byte(bad.text)); err == nil {
			t.Errorf("%s %q: no error", bad.name, bad.text)
		}
	}
}

func TestTDR(t *testing.T) {
	// A 100 ohm line, 1 ns long, between 50 ohm ports: the TDR reads 50
	// ohm, then 100 ohm until the far end's reflection comes back at 2 ns.
	var s11 []complex128
	for _, f := range grid() {
		r, _ := line(100, 1e-9, 50, f)
		s11 = append(s11, r)
	}
	tt, z := TDR(s11, 10e6, 50)
	at := func(ps float64) float64 {
		best := 0
		for k := range tt {
			if math.Abs(tt[k]-ps*1e-12) < math.Abs(tt[best]-ps*1e-12) {
				best = k
			}
		}
		return z[best]
	}
	if v := at(500); math.Abs(v-100) > 3 {
		t.Errorf("TDR at 500 ps is %.1f ohm, want 100", v)
	}
	if v := at(1500); math.Abs(v-100) > 3 {
		t.Errorf("TDR at 1500 ps is %.1f ohm, want 100", v)
	}
	if v := at(0); v > 80 {
		t.Errorf("TDR at 0 is %.1f ohm: the step should not have risen yet", v)
	}
	// Matched: 50 ohm throughout.
	zero := make([]complex128, 100)
	_, z = TDR(zero, 10e6, 50)
	for _, v := range z {
		if math.Abs(v-50) > 1e-9 {
			t.Fatalf("matched TDR %v", v)
		}
	}
}

func TestDCGrid(t *testing.T) {
	if df, err := DCGrid(grid()); err != nil || df != 10e6 {
		t.Errorf("DCGrid = %v, %v", df, err)
	}
	if _, err := DCGrid([]float64{1e8, 3e8, 5e8}); err == nil {
		t.Error("a grid not from its step: no error")
	}
}

func f64(v float64) *float64 { return &v }

func TestChecks(t *testing.T) {
	nw := pairNetwork(60, 200e-12, 12e-12)
	ports := []string{"P1", "P2", "P3", "P4"}
	l := &Limits{Checks: []Check{
		{Name: "z_single", Kind: "impedance", Port: "P1", FromPs: 100, ToPs: 300, MinOhm: f64(55), MaxOhm: f64(65)},
		{Name: "z_diff", Kind: "impedance", Pair: []string{"P1", "P3"}, FromPs: 100, ToPs: 300, MinOhm: f64(110), MaxOhm: f64(130)},
		{Name: "il", Kind: "insertion_loss", From: "P1", To: "P2", UpTo: "3GHz", MaxDB: f64(0.5)},
		{Name: "il_diff", Kind: "insertion_loss", FromPair: []string{"P1", "P3"}, ToPair: []string{"P2", "P4"}, UpTo: "1GHz", MaxDB: f64(0.5)},
		{Name: "rl", Kind: "return_loss", Port: "P1", UpTo: "100MHz", MinDB: f64(20)},
		{Name: "rl_tight", Kind: "return_loss", Port: "P1", MinDB: f64(30)},
		{Name: "skew", Kind: "skew", Positive: []string{"P1", "P2"}, Negative: []string{"P3", "P4"}, MaxPs: f64(5)},
	}}
	res, err := Run(nw, ports, l)
	if err != nil {
		t.Fatal(err)
	}
	pass := map[string]bool{}
	for _, r := range res {
		pass[r.Name] = r.Pass
		t.Logf("%s: %s (limit %s) pass=%v", r.Name, r.Measured, r.Limit, r.Pass)
	}
	for name, want := range map[string]bool{
		"z_single": true, "z_diff": true, "il": true, "il_diff": true, "rl": true,
		// A 60 ohm line on 50 ohm ports reflects about -21 dB at worst.
		"rl_tight": false,
		// 12 ps of skew against 5.
		"skew": false,
	} {
		if pass[name] != want {
			t.Errorf("%s: pass %v, want %v", name, pass[name], want)
		}
	}
	for _, r := range res {
		var v float64
		if r.Name == "skew" {
			if _, err := fmt.Sscanf(r.Measured, "%f", &v); err != nil || math.Abs(v-12) > 0.5 {
				t.Errorf("skew measured %s, want 12 ps within 0.5", r.Measured)
			}
			var a, b float64
			fmt.Sscanf(r.Measured[strings.Index(r.Measured, "arrivals")+9:], "%f and %f", &a, &b)
			// Timed at a quarter of the step: up to half a rise
			// (about 50 ps here) before the delay.
			if a > 200 || a < 150 || math.Abs(b-a-12) > 0.5 {
				t.Errorf("arrivals %v and %v ps, want within 50 ps before 200 and 212, 12 apart", a, b)
			}
		}
	}
	if err := Verdict(res, []string{"rl_tight", "skew"}); err != nil {
		t.Errorf("Verdict with the failing checks expected: %v", err)
	}
	if err := Verdict(res, nil); err == nil || !strings.Contains(err.Error(), "skew failed") {
		t.Errorf("Verdict with nothing expected: %v", err)
	}
	if err := Verdict(res, []string{"rl_tight", "skew", "il"}); err == nil || !strings.Contains(err.Error(), "il passed") {
		t.Errorf("Verdict expecting a passing check to fail: %v", err)
	}
	if err := Verdict(res, []string{"rl_tight", "skew", "nope"}); err == nil {
		t.Error("Verdict naming no check: no error")
	}
	md := Report("x.s4p", res, []string{"skew"})
	if !strings.Contains(md, "| skew | skew |") || !strings.Contains(md, "FAIL (must fail)") {
		t.Errorf("report:\n%s", md)
	}
}

func TestLimitErrors(t *testing.T) {
	for _, bad := range []string{
		`{}`,
		`{"checks": [{"kind": "skew"}]}`,
		`{"checks": [{"name": "a", "kind": "skew"}, {"name": "a", "kind": "skew"}]}`,
		`{"checks": [{"name": "a", "kind": "skew", "bogus": 1}]}`,
	} {
		if _, err := ParseLimits([]byte(bad)); err == nil {
			t.Errorf("ParseLimits(%s): no error", bad)
		}
	}
	nw := pairNetwork(50, 1e-10, 0)
	ports := []string{"P1", "P2", "P3", "P4"}
	for _, c := range []Check{
		{Name: "a", Kind: "wobble"},
		{Name: "a", Kind: "impedance", Port: "P9", FromPs: 0, ToPs: 10, MinOhm: f64(1), MaxOhm: f64(2)},
		{Name: "a", Kind: "impedance", Port: "P1"},
		{Name: "a", Kind: "insertion_loss", From: "P1", To: "P2"},
		{Name: "a", Kind: "return_loss", Port: "P1", UpTo: "1MHz", MinDB: f64(1)},
		{Name: "a", Kind: "skew", Positive: []string{"P1"}, Negative: []string{"P3", "P4"}, MaxPs: f64(1)},
		{Name: "a", Kind: "skew", Positive: []string{"P1", "P2"}, Negative: []string{"P3", "P4"}, UpTo: "1GHz", MaxPs: f64(1)},
		{Name: "a", Kind: "return_loss", Pair: []string{"P1", "P2", "P3"}, MinDB: f64(1)},
	} {
		if _, err := Run(nw, ports, &Limits{Checks: []Check{c}}); err == nil {
			t.Errorf("%+v: no error", c)
		}
	}
	if _, err := Run(nw, ports[:3], &Limits{Checks: []Check{{Name: "a", Kind: "skew"}}}); err == nil {
		t.Error("3 names for 4 ports: no error")
	}
}

// coupledPair is a differential pair from its modes: an odd-mode line of
// zodd and an even-mode line of zeven, both of delay tau. Ports: P1 D+
// near, P2 D+ far, P3 D- near, P4 D- far.
func coupledPair(zodd, zeven, tau float64) *Network {
	nw := &Network{N: 4, Z0: 50, Freqs: grid()}
	for _, f := range nw.Freqs {
		d11, d21 := line(zodd, tau, 50, f)
		c11, c21 := line(zeven, tau, 50, f)
		// Mode S as a 2x2 per mode: [near, far].
		dm := [2][2]complex128{{d11, d21}, {d21, d11}}
		cm := [2][2]complex128{{c11, c21}, {c21, c11}}
		s := make([][]complex128, 4)
		for i := range s {
			s[i] = make([]complex128, 4)
		}
		pos := [2]int{0, 1}
		neg := [2]int{2, 3}
		for a := 0; a < 2; a++ {
			for b := 0; b < 2; b++ {
				same := (dm[a][b] + cm[a][b]) / 2
				cross := (cm[a][b] - dm[a][b]) / 2
				s[pos[a]][pos[b]], s[neg[a]][neg[b]] = same, same
				s[pos[a]][neg[b]], s[neg[a]][pos[b]] = cross, cross
			}
		}
		nw.S = append(nw.S, s)
	}
	return nw
}

func TestCoupledPair(t *testing.T) {
	nw := coupledPair(45, 70, 300e-12)
	// Sdd from the single-ended S gives back the odd mode.
	sdd := nw.Sdd([2]int{0, 2}, [2]int{0, 2})
	d11, _ := line(45, 300e-12, 50, nw.Freqs[200])
	if cmplx.Abs(sdd[200]-d11) > 1e-12 {
		t.Errorf("Sdd11 %v, want the odd mode's %v", sdd[200], d11)
	}
	ports := []string{"P1", "P2", "P3", "P4"}
	l := &Limits{Checks: []Check{
		{Name: "zdiff", Kind: "impedance", Pair: []string{"P1", "P3"}, FromPs: 150, ToPs: 450, MinOhm: f64(87), MaxOhm: f64(93)},
		{Name: "zsingle", Kind: "impedance", Port: "P1", FromPs: 150, ToPs: 450, MinOhm: f64(55), MaxOhm: f64(60)},
		// The odd mode, 45 ohm on 50, reflects at worst
		// (50^2 - 45^2) / (50^2 + 45^2) = 0.105, -19.6 dB.
		{Name: "rldiff", Kind: "return_loss", Pair: []string{"P1", "P3"}, UpTo: "2GHz", MinDB: f64(19)},
		{Name: "skew", Kind: "skew", Positive: []string{"P1", "P2"}, Negative: []string{"P3", "P4"}, MaxPs: f64(1)},
	}}
	res, err := Run(nw, ports, l)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		t.Logf("%s: %s pass=%v", r.Name, r.Measured, r.Pass)
		// zdiff is 2 x 45; a single line of the pair, driven alone (its
		// neighbour terminated), sees about 56 ohm, between the modes.
		if !r.Pass {
			t.Errorf("%s failed: %s", r.Name, r.Measured)
		}
	}
}

func TestLongLine(t *testing.T) {
	// 3 ns: the phase has turned 108 degrees at 10 MHz, and more than a
	// quarter turn sends real(S21) negative there; DC must still be +1.
	nw := pairNetwork(50, 3e-9, 0)
	if dc := DC(nw.Single(1, 0)); math.Abs(dc-1) > 1e-6 {
		t.Errorf("DC of a matched 3 ns line is %v, want 1", dc)
	}
	arr, err := Arrival(nw.Single(1, 0), 10e6)
	if err != nil || math.Abs(arr-3e-9) > 60e-12 {
		t.Errorf("arrival %v, %v; want about 3 ns", arr, err)
	}
	// A reflection behind a delay: an open 3 ns away is +1 at DC.
	open := make([]complex128, len(nw.Freqs))
	for k, f := range nw.Freqs {
		open[k] = cmplx.Rect(1, -2*math.Pi*f*6e-9)
	}
	if dc := DC(open); math.Abs(dc-1) > 1e-6 {
		t.Errorf("DC of a delayed open is %v, want 1", dc)
	}
}

func TestValidation(t *testing.T) {
	nw := pairNetwork(50, 1e-10, 0)
	ports := []string{"P1", "P2", "P3", "P4"}
	for what, c := range map[string]Check{
		"port and pair":     {Name: "a", Kind: "return_loss", Port: "P1", Pair: []string{"P1", "P3"}, MinDB: f64(1)},
		"neither":           {Name: "a", Kind: "return_loss", MinDB: f64(1)},
		"impedance up_to":   {Name: "a", Kind: "impedance", Port: "P1", UpTo: "1GHz", FromPs: 0, ToPs: 10, MinOhm: f64(1), MaxOhm: f64(2)},
		"to_pair alone":     {Name: "a", Kind: "insertion_loss", ToPair: []string{"P2", "P4"}, MaxDB: f64(1)},
		"from is to":        {Name: "a", Kind: "insertion_loss", From: "P1", To: "P1", MaxDB: f64(1)},
		"negative min_db":   {Name: "a", Kind: "return_loss", Port: "P1", MinDB: f64(-10)},
		"min over max":      {Name: "a", Kind: "impedance", Port: "P1", FromPs: 0, ToPs: 10, MinOhm: f64(60), MaxOhm: f64(40)},
		"negative from_ps":  {Name: "a", Kind: "impedance", Port: "P1", FromPs: -5, ToPs: 10, MinOhm: f64(1), MaxOhm: f64(2)},
		"pair of one port":  {Name: "a", Kind: "return_loss", Pair: []string{"P1", "P1"}, MinDB: f64(1)},
		"above the band":    {Name: "a", Kind: "return_loss", Port: "P1", UpTo: "20GHz", MinDB: f64(1)},
		"skew up_to":        {Name: "a", Kind: "skew", Positive: []string{"P1", "P2"}, Negative: []string{"P3", "P4"}, UpTo: "1GHz", MaxPs: f64(1)},
		"loss with min_ohm": {Name: "a", Kind: "insertion_loss", From: "P1", To: "P2", MaxDB: f64(1), MinOhm: f64(1)},
	} {
		if _, err := Run(nw, ports, &Limits{Checks: []Check{c}}); err == nil {
			t.Errorf("%s: no error", what)
		}
	}
	for _, bad := range []string{
		"# Hz S RI\n1 NaN 0\n",
		"# Hz S RI R NaN\n1 0 0\n",
		"1 0 0\n# Hz S RI\n2 0 0\n",
	} {
		if _, err := ParseTouchstone("x.s1p", []byte(bad)); err == nil {
			t.Errorf("ParseTouchstone(%q): no error", bad)
		}
	}
	// A 3000-point grid written with 6 digits, as %g does: still k * df.
	var fs []float64
	for k := 1; k <= 3000; k++ {
		v, _ := strconvParse(fmt.Sprintf("%.6g", float64(k)*1e10/3000))
		fs = append(fs, v)
	}
	if _, err := DCGrid(fs); err != nil {
		t.Errorf("a rounded 3000-point grid: %v", err)
	}
}

func strconvParse(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%g", &v)
	return v, err
}
