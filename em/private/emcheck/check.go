// SPDX-License-Identifier: Apache-2.0

// Package emcheck is kicad_em_test's checker (issue #1, step 4): it reads
// a kicad_em_sim's S-parameters and checks them against limits, per port
// or per differential pair: impedance (from the TDR), insertion loss,
// return loss and skew.
package emcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Check is one limit, as the limits file gives it.
type Check struct {
	Name string `json:"name"`
	Kind string `json:"kind"`

	// impedance and return_loss: one port, or a differential pair
	// (positive, negative) at the same end.
	Port string   `json:"port,omitempty"`
	Pair []string `json:"pair,omitempty"`

	// insertion_loss: from and to a port, or from and to a pair.
	From     string   `json:"from,omitempty"`
	To       string   `json:"to,omitempty"`
	FromPair []string `json:"from_pair,omitempty"`
	ToPair   []string `json:"to_pair,omitempty"`

	// skew: the positive and negative lines, each (near port, far port);
	// their arrivals (half of the step through each) are compared.
	Positive []string `json:"positive,omitempty"`
	Negative []string `json:"negative,omitempty"`

	// The band checked: up to this frequency, such as "3GHz".
	UpTo string `json:"up_to,omitempty"`
	// impedance: the TDR's time window, in ps, and the impedance range.
	FromPs float64  `json:"from_ps,omitempty"`
	ToPs   float64  `json:"to_ps,omitempty"`
	MinOhm *float64 `json:"min_ohm,omitempty"`
	MaxOhm *float64 `json:"max_ohm,omitempty"`
	// insertion_loss: the most loss allowed; return_loss: the least.
	MaxDB *float64 `json:"max_db,omitempty"`
	MinDB *float64 `json:"min_db,omitempty"`
	// skew: the most allowed.
	MaxPs *float64 `json:"max_ps,omitempty"`
}

// Limits is a limits file.
type Limits struct {
	Checks []Check `json:"checks"`
}

// Result is one check's outcome.
type Result struct {
	Name, Kind, Measured, Limit string
	Pass                        bool
}

var hzRe = regexp.MustCompile(`^\s*([0-9.eE+-]+)\s*(Hz|kHz|MHz|GHz)\s*$`)

// Hz reads "3GHz", "100 MHz".
func Hz(s string) (float64, error) {
	m := hzRe.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("frequency %q: want a number and Hz, kHz, MHz or GHz", s)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil || v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, fmt.Errorf("frequency %q: want above 0", s)
	}
	return v * map[string]float64{"Hz": 1, "kHz": 1e3, "MHz": 1e6, "GHz": 1e9}[m[2]], nil
}

// ParseLimits reads a limits file, refusing unknown fields.
func ParseLimits(data []byte) (*Limits, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var l Limits
	if err := d.Decode(&l); err != nil {
		return nil, fmt.Errorf("limits: %v", err)
	}
	if len(l.Checks) == 0 {
		return nil, fmt.Errorf("limits: no checks")
	}
	names := map[string]bool{}
	for _, c := range l.Checks {
		if c.Name == "" {
			return nil, fmt.Errorf("limits: a %s check with no name", c.Kind)
		}
		if names[c.Name] {
			return nil, fmt.Errorf("limits: two checks named %s", c.Name)
		}
		names[c.Name] = true
	}
	return &l, nil
}

type evaluator struct {
	nw    *Network
	ports map[string]int
}

func (e *evaluator) port(name string) (int, error) {
	i, ok := e.ports[name]
	if !ok {
		var have []string
		for p := range e.ports {
			have = append(have, p)
		}
		sort.Strings(have)
		return 0, fmt.Errorf("no port %q; the simulation has %s", name, strings.Join(have, ", "))
	}
	return i, nil
}

func (e *evaluator) pair(names []string) ([2]int, error) {
	if len(names) != 2 {
		return [2]int{}, fmt.Errorf("a pair is two ports, positive and negative, not %d", len(names))
	}
	p, err := e.port(names[0])
	if err != nil {
		return [2]int{}, err
	}
	n, err := e.port(names[1])
	return [2]int{p, n}, err
}

// upTo is the index past the last frequency at or below the check's
// up_to (all of them if it has none).
func (e *evaluator) upTo(c Check) (int, float64, error) {
	if c.UpTo == "" {
		return len(e.nw.Freqs), e.nw.Freqs[len(e.nw.Freqs)-1], nil
	}
	f, err := Hz(c.UpTo)
	if err != nil {
		return 0, 0, err
	}
	k := sort.SearchFloat64s(e.nw.Freqs, f*(1+1e-9))
	if k == 0 {
		return 0, 0, fmt.Errorf("up_to %s is below the band's first frequency, %g Hz", c.UpTo, e.nw.Freqs[0])
	}
	return k, f, nil
}

func ohm(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) + " ohm" }
func db(v float64) string  { return strconv.FormatFloat(v, 'f', 2, 64) + " dB" }

func (e *evaluator) check(c Check) (Result, error) {
	r := Result{Name: c.Name, Kind: c.Kind}
	switch c.Kind {
	case "impedance":
		if c.MinOhm == nil || c.MaxOhm == nil || c.ToPs <= c.FromPs {
			return r, fmt.Errorf("impedance needs min_ohm, max_ohm, and from_ps below to_ps")
		}
		df, err := DCGrid(e.nw.Freqs)
		if err != nil {
			return r, err
		}
		var s11 []complex128
		z0 := e.nw.Z0
		what := c.Port
		if c.Pair != nil {
			p, err := e.pair(c.Pair)
			if err != nil {
				return r, err
			}
			s11, z0 = e.nw.Mixed("dd", p, p), 2*e.nw.Z0
			what = "differential " + strings.Join(c.Pair, "/")
		} else {
			i, err := e.port(c.Port)
			if err != nil {
				return r, err
			}
			s11 = e.nw.Single(i, i)
		}
		t, z := TDR(s11, df, z0)
		lo, hi := math.Inf(1), math.Inf(-1)
		for k := range t {
			ps := t[k] * 1e12
			if ps >= c.FromPs && ps <= c.ToPs {
				lo, hi = math.Min(lo, z[k]), math.Max(hi, z[k])
			}
		}
		if math.IsInf(lo, 1) {
			return r, fmt.Errorf("no TDR sample from %g to %g ps", c.FromPs, c.ToPs)
		}
		r.Measured = fmt.Sprintf("%s: %s to %s, %g to %g ps", what, ohm(lo), ohm(hi), c.FromPs, c.ToPs)
		r.Limit = fmt.Sprintf("%s to %s", ohm(*c.MinOhm), ohm(*c.MaxOhm))
		r.Pass = lo >= *c.MinOhm && hi <= *c.MaxOhm
	case "insertion_loss", "return_loss":
		end, f, err := e.upTo(c)
		if err != nil {
			return r, err
		}
		var s []complex128
		var what string
		switch {
		case c.Kind == "insertion_loss" && c.FromPair != nil:
			a, err := e.pair(c.FromPair)
			if err != nil {
				return r, err
			}
			b, err := e.pair(c.ToPair)
			if err != nil {
				return r, err
			}
			s, what = e.nw.Mixed("dd", b, a), "Sdd "+strings.Join(c.ToPair, "/")+" from "+strings.Join(c.FromPair, "/")
		case c.Kind == "insertion_loss":
			a, err := e.port(c.From)
			if err != nil {
				return r, err
			}
			b, err := e.port(c.To)
			if err != nil {
				return r, err
			}
			s, what = e.nw.Single(b, a), "S "+c.To+" from "+c.From
		case c.Pair != nil:
			p, err := e.pair(c.Pair)
			if err != nil {
				return r, err
			}
			s, what = e.nw.Mixed("dd", p, p), "Sdd "+strings.Join(c.Pair, "/")
		default:
			i, err := e.port(c.Port)
			if err != nil {
				return r, err
			}
			s, what = e.nw.Single(i, i), "S "+c.Port
		}
		// The worst frequency: the most loss through, or the least loss
		// back.
		worst, at := 0.0, 0.0
		for k := 0; k < end; k++ {
			v := -DB(s[k])
			if k == 0 || (c.Kind == "insertion_loss" && v > worst) || (c.Kind == "return_loss" && v < worst) {
				worst, at = v, e.nw.Freqs[k]
			}
		}
		r.Measured = fmt.Sprintf("%s: %s at %.4g GHz (up to %.4g GHz)", what, db(worst), at/1e9, f/1e9)
		if c.Kind == "insertion_loss" {
			if c.MaxDB == nil {
				return r, fmt.Errorf("insertion_loss needs max_db")
			}
			r.Limit, r.Pass = "at most "+db(*c.MaxDB), worst <= *c.MaxDB
		} else {
			if c.MinDB == nil {
				return r, fmt.Errorf("return_loss needs min_db")
			}
			r.Limit, r.Pass = "at least "+db(*c.MinDB), worst >= *c.MinDB
		}
	case "skew":
		if c.MaxPs == nil {
			return r, fmt.Errorf("skew needs max_ps")
		}
		if c.UpTo != "" {
			return r, fmt.Errorf("skew uses the whole band (its step's rise), so it takes no up_to")
		}
		df, err := DCGrid(e.nw.Freqs)
		if err != nil {
			return r, err
		}
		pos, err := e.pair(c.Positive)
		if err != nil {
			return r, fmt.Errorf("positive: %v", err)
		}
		neg, err := e.pair(c.Negative)
		if err != nil {
			return r, fmt.Errorf("negative: %v", err)
		}
		tp, err := Arrival(e.nw.Single(pos[1], pos[0]), df)
		if err != nil {
			return r, fmt.Errorf("positive: %v", err)
		}
		tn, err := Arrival(e.nw.Single(neg[1], neg[0]), df)
		if err != nil {
			return r, fmt.Errorf("negative: %v", err)
		}
		skew := math.Abs(tp-tn) * 1e12
		r.Measured = fmt.Sprintf("%.2f ps (arrivals %.2f and %.2f ps)", skew, tp*1e12, tn*1e12)
		r.Limit = fmt.Sprintf("at most %.2f ps", *c.MaxPs)
		r.Pass = skew <= *c.MaxPs
	default:
		return r, fmt.Errorf("kind %q: want impedance, insertion_loss, return_loss or skew", c.Kind)
	}
	return r, nil
}

// Run checks a network, its ports named in Touchstone order, against
// limits.
func Run(nw *Network, ports []string, l *Limits) ([]Result, error) {
	if len(ports) != nw.N {
		return nil, fmt.Errorf("%d port names for a %d-port network", len(ports), nw.N)
	}
	e := &evaluator{nw: nw, ports: map[string]int{}}
	for i, p := range ports {
		e.ports[p] = i
	}
	var out []Result
	for _, c := range l.Checks {
		r, err := e.check(c)
		if err != nil {
			return nil, fmt.Errorf("check %s: %v", c.Name, err)
		}
		out = append(out, r)
	}
	return out, nil
}

// Verdict is nil when every check passes but those in expectFail, and
// those fail; else it says which did not go as expected.
func Verdict(results []Result, expectFail []string) error {
	want := map[string]bool{}
	for _, n := range expectFail {
		want[n] = true
	}
	known := map[string]bool{}
	var bad []string
	for _, r := range results {
		known[r.Name] = true
		switch {
		case want[r.Name] && r.Pass:
			bad = append(bad, r.Name+" passed, but this board must fail it")
		case !want[r.Name] && !r.Pass:
			bad = append(bad, r.Name+" failed: "+r.Measured+", limit "+r.Limit)
		}
	}
	for _, n := range expectFail {
		if !known[n] {
			bad = append(bad, "expect_fail names "+n+", which is not a check")
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%s", strings.Join(bad, "; "))
	}
	return nil
}

// Report is the results as Markdown.
func Report(title string, results []Result, expectFail []string) string {
	want := map[string]bool{}
	for _, n := range expectFail {
		want[n] = true
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n| Check | Kind | Measured | Limit | Result |\n|---|---|---|---|---|\n", title)
	for _, r := range results {
		res := "pass"
		if !r.Pass {
			res = "FAIL"
		}
		if want[r.Name] {
			res += " (must fail)"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", r.Name, r.Kind, r.Measured, r.Limit, res)
	}
	return b.String()
}
