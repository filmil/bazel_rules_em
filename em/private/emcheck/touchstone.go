// SPDX-License-Identifier: Apache-2.0

package emcheck

import (
	"fmt"
	"math"
	"math/cmplx"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Network is an N-port's S-parameters on a frequency grid.
type Network struct {
	N     int
	Freqs []float64        // Hz
	S     [][][]complex128 // S[k][i][j]: frequency k, from port j to port i
	Z0    float64
}

var portsRe = regexp.MustCompile(`(?i)\.s(\d+)p$`)

// ParseTouchstone reads a Touchstone 1.1 file of S-parameters: its
// option line (frequency unit, S, RI, MA or DB, R), and N ports, N from
// the file's name (.s<N>p).
func ParseTouchstone(name string, data []byte) (*Network, error) {
	m := portsRe.FindStringSubmatch(filepath.Base(name))
	if m == nil {
		return nil, fmt.Errorf("%s: not a .s<N>p file", name)
	}
	n, _ := strconv.Atoi(m[1])
	if n < 1 {
		return nil, fmt.Errorf("%s: %d ports", name, n)
	}
	unit, format, z0 := 1e9, "MA", 50.0 // Touchstone's defaults: GHz, MA, 50 ohm
	var nums []float64
	seenOption := false
	for ln, line := range strings.Split(string(data), "\n") {
		if i := strings.IndexByte(line, '!'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if seenOption {
				return nil, fmt.Errorf("%s:%d: a second option line", name, ln+1)
			}
			if len(nums) > 0 {
				return nil, fmt.Errorf("%s:%d: the option line after data", name, ln+1)
			}
			seenOption = true
			f := strings.Fields(strings.ToUpper(line[1:]))
			for i := 0; i < len(f); i++ {
				switch f[i] {
				case "HZ":
					unit = 1
				case "KHZ":
					unit = 1e3
				case "MHZ":
					unit = 1e6
				case "GHZ":
					unit = 1e9
				case "S":
				case "Y", "Z", "H", "G":
					return nil, fmt.Errorf("%s:%d: %s-parameters: only S is read", name, ln+1, f[i])
				case "RI", "MA", "DB":
					format = f[i]
				case "R":
					if i+1 >= len(f) {
						return nil, fmt.Errorf("%s:%d: R with no value", name, ln+1)
					}
					v, err := strconv.ParseFloat(f[i+1], 64)
					if err != nil || !(v > 0) || math.IsInf(v, 0) {
						return nil, fmt.Errorf("%s:%d: R %q", name, ln+1, f[i+1])
					}
					z0 = v
					i++
				default:
					return nil, fmt.Errorf("%s:%d: option %q not known", name, ln+1, f[i])
				}
			}
			continue
		}
		for _, x := range strings.Fields(line) {
			v, err := strconv.ParseFloat(x, 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("%s:%d: %q is not a finite number", name, ln+1, x)
			}
			nums = append(nums, v)
		}
	}
	per := 1 + 2*n*n
	if len(nums) == 0 || len(nums)%per != 0 {
		return nil, fmt.Errorf("%s: %d numbers, not a whole number of %d-port records of %d", name, len(nums), n, per)
	}
	nw := &Network{N: n, Z0: z0}
	for r := 0; r < len(nums); r += per {
		rec := nums[r : r+per]
		f := rec[0] * unit
		if len(nw.Freqs) > 0 && f <= nw.Freqs[len(nw.Freqs)-1] {
			return nil, fmt.Errorf("%s: frequency %g Hz does not rise", name, f)
		}
		nw.Freqs = append(nw.Freqs, f)
		s := make([][]complex128, n)
		for i := range s {
			s[i] = make([]complex128, n)
		}
		for k := 0; k < n*n; k++ {
			a, b := rec[1+2*k], rec[2+2*k]
			var c complex128
			switch format {
			case "RI":
				c = complex(a, b)
			case "MA":
				c = cmplx.Rect(a, b*math.Pi/180)
			case "DB":
				c = cmplx.Rect(math.Pow(10, a/20), b*math.Pi/180)
			}
			// Touchstone 1's two-port order is S11 S21 S12 S22; every
			// other size is row by row.
			i, j := k/n, k%n
			if n == 2 {
				i, j = k%2, k/2
			}
			s[i][j] = c
		}
		nw.S = append(nw.S, s)
	}
	return nw, nil
}
