// SPDX-License-Identifier: Apache-2.0

// Package pcbmodel makes kicad_em_model's model of a board (issue #1,
// step 2): the copper of the nets to analyse, from the .kicad_pcb itself
// so that every shape keeps its net's name, with the stackup and the
// ports. It writes model.json, which the openEMS script turns into
// CSXCAD geometry, and model.png, a picture of it.
package pcbmodel

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Options says what to model.
type Options struct {
	// Nets are the nets to model; their copper, and nothing else, is in
	// the model.
	Nets []string
	// Ports maps a port's name to a pad, "REF.PAD", such as "J1.A6".
	Ports map[string]string
	// Copper is each copper layer's thickness, mm.
	Copper float64
	// Dielectrics are the layers between the copper, top first: one fewer
	// than the copper layers.
	Dielectrics []Dielectric
}

// Dielectric is one dielectric layer.
type Dielectric struct {
	Thickness, EpsilonR, LossTangent float64
}

var dielectricRe = regexp.MustCompile(`^\s*([0-9.]+)\s*mm\s+([0-9.]+)\s+([0-9.]+)\s*$`)

// ParseDielectric reads "1.51mm 4.5 0.02": thickness, relative
// permittivity, loss tangent.
func ParseDielectric(s string) (Dielectric, error) {
	m := dielectricRe.FindStringSubmatch(s)
	if m == nil {
		return Dielectric{}, fmt.Errorf("dielectric %q: want \"<thickness>mm <epsilon_r> <loss tangent>\", such as \"1.51mm 4.5 0.02\"", s)
	}
	d := Dielectric{}
	d.Thickness, _ = strconv.ParseFloat(m[1], 64)
	d.EpsilonR, _ = strconv.ParseFloat(m[2], 64)
	d.LossTangent, _ = strconv.ParseFloat(m[3], 64)
	if d.Thickness <= 0 || d.EpsilonR < 1 {
		return Dielectric{}, fmt.Errorf("dielectric %q: the thickness must be above 0 and epsilon_r at least 1", s)
	}
	return d, nil
}

// Layer is a copper or dielectric layer of the model, Z up from the
// bottom copper's bottom face.
type Layer struct {
	Name           string
	Z, Thickness   float64
	EpsilonR, Tand float64 // dielectrics only
}

// Port is a pad the simulation excites or measures.
type Port struct {
	Name, Pad, Net, Layer string
	At                    Pt
}

// Model is what kicad_em_model writes, in the model's frame: mm, X as
// KiCad's, Y up (KiCad's Y negated), Z up from the bottom copper.
type Model struct {
	Board       [4]float64 // the outline's box: min X, min Y, max X, max Y
	Thickness   float64    // the board's thickness as the file gives it
	Copper      []Layer
	Dielectrics []Layer
	Nets        []string
	Shapes      []Shape
	Holes       []Hole
	Ports       []Port
}

func flip(p Pt) Pt { return Pt{p.X, -p.Y} }

// Build selects the model's copper from the board.
func Build(b *Board, o Options) (*Model, error) {
	if len(o.Nets) == 0 {
		return nil, fmt.Errorf("no nets to model")
	}
	want := map[string]bool{}
	for _, n := range o.Nets {
		if !b.Nets[n] {
			var have []string
			for k := range b.Nets {
				have = append(have, k)
			}
			sort.Strings(have)
			return nil, fmt.Errorf("net %q is not on the board; it has %s", n, strings.Join(have, ", "))
		}
		want[n] = true
	}
	if len(o.Dielectrics) != len(b.Layers)-1 {
		return nil, fmt.Errorf("the board has %d copper layers, so %d dielectrics, not %d",
			len(b.Layers), len(b.Layers)-1, len(o.Dielectrics))
	}
	if o.Copper <= 0 {
		return nil, fmt.Errorf("copper thickness %v mm: want above 0", o.Copper)
	}
	m := &Model{
		Board:     [4]float64{b.Outline[0], -b.Outline[3], b.Outline[2], -b.Outline[1]},
		Thickness: b.Thickness,
	}
	nets := append([]string(nil), o.Nets...)
	sort.Strings(nets)
	m.Nets = nets

	// From the bottom up: B.Cu, the dielectric above it, ..., F.Cu.
	z := 0.0
	cu := map[string]Layer{}
	for i := len(b.Layers) - 1; i >= 0; i-- {
		l := Layer{Name: b.Layers[i], Z: z, Thickness: o.Copper}
		cu[l.Name] = l
		z += o.Copper
		if i > 0 {
			d := o.Dielectrics[i-1]
			m.Dielectrics = append([]Layer{{
				Name: fmt.Sprintf("dielectric %d", i), Z: z, Thickness: d.Thickness,
				EpsilonR: d.EpsilonR, Tand: d.LossTangent,
			}}, m.Dielectrics...)
			z += d.Thickness
		}
	}
	for _, name := range b.Layers {
		m.Copper = append(m.Copper, cu[name])
	}

	for _, s := range b.Shapes {
		if !want[s.Net] {
			continue
		}
		if _, ok := cu[s.Layer]; !ok {
			return nil, fmt.Errorf("net %s has copper on %s, not a copper layer", s.Net, s.Layer)
		}
		pts := make([]Pt, len(s.Points))
		for k, p := range s.Points {
			pts[k] = flip(p)
		}
		m.Shapes = append(m.Shapes, Shape{s.Layer, s.Net, pts})
	}
	for _, h := range b.Holes {
		if want[h.Net] {
			h.At = flip(h.At)
			m.Holes = append(m.Holes, h)
		}
	}
	var names []string
	for n := range o.Ports {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		ref := o.Ports[n]
		p, ok := b.Pads[ref]
		if !ok {
			return nil, fmt.Errorf("port %s: no pad %s on the board (want REF.PAD, such as J1.A6)", n, ref)
		}
		if !want[p.Net] {
			return nil, fmt.Errorf("port %s: pad %s is on net %q, which is not one of the nets modelled", n, ref, p.Net)
		}
		m.Ports = append(m.Ports, Port{n, ref, p.Net, p.Layers[0], flip(p.At)})
	}
	if len(m.Shapes) == 0 {
		return nil, fmt.Errorf("the nets %s have no copper", strings.Join(nets, ", "))
	}
	return m, nil
}

// f writes a number in mm with 6 decimals (a nanometre), so the JSON does
// not depend on how a float prints.
func f(v float64) string {
	if v == 0 || math.Abs(v) < 5e-7 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', 6, 64)
}

func q(s string) string { return strconv.Quote(s) }

// JSON writes the model, in a fixed order.
func (m *Model) JSON() []byte {
	var b bytes.Buffer
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	p("{\n  \"units\": \"mm\",\n")
	p("  \"frame\": \"X as KiCad's, Y up (KiCad's Y negated), Z up from the bottom copper's bottom face\",\n")
	p("  \"board\": {\"min_x\": %s, \"min_y\": %s, \"max_x\": %s, \"max_y\": %s, \"thickness\": %s},\n",
		f(m.Board[0]), f(m.Board[1]), f(m.Board[2]), f(m.Board[3]), f(m.Thickness))
	p("  \"copper\": [")
	for i, l := range m.Copper {
		if i > 0 {
			p(",")
		}
		p("\n    {\"name\": %s, \"z\": %s, \"thickness\": %s}", q(l.Name), f(l.Z), f(l.Thickness))
	}
	p("\n  ],\n  \"dielectrics\": [")
	for i, l := range m.Dielectrics {
		if i > 0 {
			p(",")
		}
		p("\n    {\"name\": %s, \"z\": %s, \"thickness\": %s, \"epsilon_r\": %s, \"loss_tangent\": %s}",
			q(l.Name), f(l.Z), f(l.Thickness), f(l.EpsilonR), f(l.Tand))
	}
	p("\n  ],\n  \"nets\": [")
	for i, n := range m.Nets {
		if i > 0 {
			p(", ")
		}
		p("%s", q(n))
	}
	p("],\n  \"ports\": [")
	for i, pt := range m.Ports {
		if i > 0 {
			p(",")
		}
		p("\n    {\"name\": %s, \"pad\": %s, \"net\": %s, \"layer\": %s, \"x\": %s, \"y\": %s}",
			q(pt.Name), q(pt.Pad), q(pt.Net), q(pt.Layer), f(pt.At.X), f(pt.At.Y))
	}
	p("\n  ],\n  \"holes\": [")
	for i, h := range m.Holes {
		if i > 0 {
			p(",")
		}
		var ls []string
		for _, l := range h.Layers {
			ls = append(ls, q(l))
		}
		p("\n    {\"net\": %s, \"x\": %s, \"y\": %s, \"drill\": %s, \"size\": %s, \"layers\": [%s]}",
			q(h.Net), f(h.At.X), f(h.At.Y), f(h.Drill), f(h.Size), strings.Join(ls, ", "))
	}
	p("\n  ],\n  \"shapes\": [")
	for i, s := range m.Shapes {
		if i > 0 {
			p(",")
		}
		p("\n    {\"layer\": %s, \"net\": %s, \"points\": [", q(s.Layer), q(s.Net))
		for k, pt := range s.Points {
			if k > 0 {
				p(", ")
			}
			p("[%s, %s]", f(pt.X), f(pt.Y))
		}
		p("]}")
	}
	p("\n  ]\n}\n")
	return b.Bytes()
}
