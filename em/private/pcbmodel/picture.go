// SPDX-License-Identifier: Apache-2.0

package pcbmodel

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"sort"
)

// Picture colours.
var (
	white   = color.NRGBA{0xff, 0xff, 0xff, 0xff}
	boardBg = color.NRGBA{0xee, 0xea, 0xdc, 0xff}
	top     = color.NRGBA{0xc8, 0x3c, 0x2c, 0xff}
	bottom  = color.NRGBA{0x2c, 0x5c, 0xc8, 0xff}
	both    = color.NRGBA{0x7c, 0x3c, 0x8c, 0xff}
	inner   = color.NRGBA{0x3c, 0x9c, 0x4c, 0xff}
	drill   = color.NRGBA{0x20, 0x20, 0x20, 0xff}
	portMrk = color.NRGBA{0xf0, 0xb0, 0x00, 0xff}
)

const ss = 2 // samples per pixel, each way

type grid struct {
	minX, maxY float64
	scale      float64 // samples per mm
	w, h       int
	bits       []bool
}

func newGrid(m *Model, pxPerMM int) *grid {
	const margin = 1.0
	g := &grid{minX: m.Board[0] - margin, maxY: m.Board[3] + margin, scale: float64(pxPerMM * ss)}
	g.w = int(math.Ceil((m.Board[2]-m.Board[0]+2*margin)*float64(pxPerMM))) * ss
	g.h = int(math.Ceil((m.Board[3]-m.Board[1]+2*margin)*float64(pxPerMM))) * ss
	g.bits = make([]bool, g.w*g.h)
	return g
}

// fill sets the samples inside a polygon (nonzero rule).
func (g *grid) fill(poly []Pt) {
	type cross struct {
		x   float64
		dir int
	}
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, p := range poly {
		minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
	}
	j0 := int(math.Max(0, math.Floor((g.maxY-maxY)*g.scale)))
	j1 := int(math.Min(float64(g.h), math.Ceil((g.maxY-minY)*g.scale)+1))
	var xs []cross
	for j := j0; j < j1; j++ {
		y := g.maxY - (float64(j)+0.5)/g.scale
		xs = xs[:0]
		for k := range poly {
			a, b := poly[k], poly[(k+1)%len(poly)]
			switch {
			case a.Y <= y && b.Y > y:
				xs = append(xs, cross{a.X + (y-a.Y)*(b.X-a.X)/(b.Y-a.Y), 1})
			case b.Y <= y && a.Y > y:
				xs = append(xs, cross{a.X + (y-a.Y)*(b.X-a.X)/(b.Y-a.Y), -1})
			}
		}
		sort.Slice(xs, func(p, q int) bool {
			if xs[p].x != xs[q].x {
				return xs[p].x < xs[q].x
			}
			return xs[p].dir < xs[q].dir
		})
		wind := 0
		for k := 0; k+1 < len(xs); k++ {
			wind += xs[k].dir
			if wind == 0 {
				continue
			}
			from := int(math.Ceil((xs[k].x-g.minX)*g.scale - 0.5))
			to := int(math.Ceil((xs[k+1].x-g.minX)*g.scale - 0.5))
			for i := max(from, 0); i < min(to, g.w); i++ {
				g.bits[j*g.w+i] = true
			}
		}
	}
}

// Picture draws the model from the top: the board's box, top copper red,
// bottom copper blue (purple where both), inner copper green, holes dark,
// and each port as an amber ring.
func (m *Model) Picture(pxPerMM int) ([]byte, error) {
	if pxPerMM < 1 || pxPerMM > maxPxPerMM {
		return nil, fmt.Errorf("%d pixels per mm: want 1 to %d", pxPerMM, maxPxPerMM)
	}
	layer := func(name string) *grid {
		g := newGrid(m, pxPerMM)
		for _, s := range m.Shapes {
			if s.Layer == name || name == "inner" && s.Layer != "F.Cu" && s.Layer != "B.Cu" {
				g.fill(s.Points)
			}
		}
		return g
	}
	fcu, bcu, icu := layer("F.Cu"), layer("B.Cu"), layer("inner")
	area, holes, ports := newGrid(m, pxPerMM), newGrid(m, pxPerMM), newGrid(m, pxPerMM)
	area.fill([]Pt{{m.Board[0], m.Board[1]}, {m.Board[2], m.Board[1]}, {m.Board[2], m.Board[3]}, {m.Board[0], m.Board[3]}})
	for _, h := range m.Holes {
		if h.Slot != nil {
			holes.fill(h.Slot)
		} else {
			holes.fill(circle(h.At, h.Drill/2))
		}
	}
	for _, p := range m.Ports {
		outer, in := circle(p.At, 0.5), circle(p.At, 0.3)
		// A ring: the outer circle, then the inner one the other way round.
		for l, r := 0, len(in)-1; l < r; l, r = l+1, r-1 {
			in[l], in[r] = in[r], in[l]
		}
		ring := append(append([]Pt{}, outer...), outer[0])
		ring = append(ring, in[len(in)-1])
		ring = append(ring, in...)
		ports.fill(ring)
	}
	w, h := fcu.w/ss, fcu.h/ss
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					i := (y*ss+sy)*fcu.w + x*ss + sx
					c := white
					switch {
					case ports.bits[i]:
						c = portMrk
					case holes.bits[i]:
						c = drill
					case fcu.bits[i] && bcu.bits[i]:
						c = both
					case fcu.bits[i]:
						c = top
					case bcu.bits[i]:
						c = bottom
					case icu.bits[i]:
						c = inner
					case area.bits[i]:
						c = boardBg
					}
					r, g, b = r+int(c.R), g+int(c.G), b+int(c.B)
				}
			}
			n := ss * ss
			img.SetNRGBA(x, y, color.NRGBA{uint8(r / n), uint8(g / n), uint8(b / n), 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
