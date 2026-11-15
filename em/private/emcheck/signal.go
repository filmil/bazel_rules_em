// SPDX-License-Identifier: Apache-2.0

package emcheck

import (
	"fmt"
	"math"
	"math/cmplx"
)

// TDRPad is how many times the TDR's spectrum is padded with zeros, as in
// kicad_em_sim's sim.py, so the two draw the same step.
const TDRPad = 16

// fft is an in-place radix-2 FFT; len(a) must be a power of 2. inverse
// gives the unscaled inverse transform.
func fft(a []complex128, inverse bool) {
	n := len(a)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	sign := -1.0
	if inverse {
		sign = 1
	}
	for size := 2; size <= n; size <<= 1 {
		w := cmplx.Rect(1, sign*2*math.Pi/float64(size))
		for start := 0; start < n; start += size {
			t := complex(1, 0)
			for k := 0; k < size/2; k++ {
				u, v := a[start+k], a[start+k+size/2]*t
				a[start+k], a[start+k+size/2] = u+v, u-v
				t *= w
			}
		}
	}
}

// DCGrid checks that freqs are k * df for k = 1, 2, ... (df = freqs[0]),
// within 0.1 %, and returns df.
func DCGrid(freqs []float64) (float64, error) {
	if len(freqs) < 2 {
		return 0, fmt.Errorf("%d frequencies: the TDR needs a grid", len(freqs))
	}
	df := freqs[0]
	for k, f := range freqs {
		if math.Abs(f-float64(k+1)*df) > 1e-3*df {
			return 0, fmt.Errorf("frequency %d is %g Hz, not %d x %g: the TDR needs a grid that starts at "+
				"its step (such as f_start 10MHz, f_stop 10GHz, points 1000)", k+1, f, k+1, df)
		}
	}
	return df, nil
}

// Step is the response to a Hann-windowed step of a parameter s given
// on df, 2 df, ... (DC is taken as s at df, made real): time in s and the
// response, from t = 0. It is kicad_em_sim's TDR step: a reflection gives
// the TDR, a transmission the TDT.
func Step(s []complex128, df float64) ([]float64, []float64) {
	spec := append([]complex128{complex(real(s[0]), 0)}, s...)
	n := len(spec)
	// The output length: the next power of two at or above 2 (n - 1) pad.
	m := 1
	for m < 2*(n-1)*TDRPad {
		m <<= 1
	}
	// Hermitian full spectrum of length m, the band windowed by a falling
	// half Hann window, zeros above it.
	full := make([]complex128, m)
	for k := 0; k < n; k++ {
		w := 0.5 * (1 + math.Cos(math.Pi*float64(k)/float64(n)))
		full[k] = spec[k] * complex(w, 0)
		if k > 0 && k < m/2 {
			full[m-k] = cmplx.Conj(full[k])
		}
	}
	fft(full, true)
	// The impulse spreads both sides of t = 0: integrate from the middle
	// of the wrapped half before it.
	sum := 0.0
	for i := m / 2; i < m; i++ { // negative times
		sum += real(full[i]) / float64(m)
	}
	t := make([]float64, m/2)
	step := make([]float64, m/2)
	dt := 1.0 / (float64(m) * df)
	for i := 0; i < m/2; i++ {
		sum += real(full[i]) / float64(m)
		t[i] = float64(i) * dt
		step[i] = sum
	}
	return t, step
}

// TDR is the impedance seen by a step through a reflection s11: Z0 (1 +
// r) / (1 - r) of its step response.
func TDR(s11 []complex128, df, z0 float64) ([]float64, []float64) {
	t, r := Step(s11, df)
	z := make([]float64, len(r))
	for i, v := range r {
		v = math.Max(-0.999, math.Min(0.999, v))
		z[i] = z0 * (1 + v) / (1 - v)
	}
	return t, z
}

// Arrival is when a transmission's step response first reaches half of
// its final value: the line's delay, measured on its first edge, before
// any reflection from a mismatch arrives.
func Arrival(s21 []complex128, df float64) (float64, error) {
	t, v := Step(s21, df)
	final := real(s21[0]) // at the lowest frequency, about DC
	if math.Abs(final) < 0.1 {
		return 0, fmt.Errorf("the transmission at %g Hz is %.3f: too little gets through to time it", df, final)
	}
	half := final / 2
	for i := 1; i < len(v); i++ {
		if (v[i-1]-half)*(v[i]-half) <= 0 && v[i] != v[i-1] {
			// Linear between the two samples.
			return t[i-1] + (half-v[i-1])/(v[i]-v[i-1])*(t[i]-t[i-1]), nil
		}
	}
	return 0, fmt.Errorf("the step never reaches half its final value")
}

// Single is the reflection or transmission from port j to port i.
func (nw *Network) Single(i, j int) []complex128 {
	out := make([]complex128, len(nw.Freqs))
	for k := range out {
		out[k] = nw.S[k][i][j]
	}
	return out
}

// Mixed is a mixed-mode parameter between two pairs, each (positive,
// negative) port: dd differential to differential, cd differential to
// common.
func (nw *Network) Mixed(kind string, to, from [2]int) []complex128 {
	out := make([]complex128, len(nw.Freqs))
	p2, n2 := to[0], to[1]
	p1, n1 := from[0], from[1]
	for k := range out {
		s := nw.S[k]
		switch kind {
		case "dd":
			out[k] = (s[p2][p1] - s[p2][n1] - s[n2][p1] + s[n2][n1]) / 2
		case "cd":
			out[k] = (s[p2][p1] - s[p2][n1] + s[n2][p1] - s[n2][n1]) / 2
		}
	}
	return out
}

// DB is 20 log10 |c|.
func DB(c complex128) float64 { return 20 * math.Log10(math.Max(cmplx.Abs(c), 1e-15)) }
