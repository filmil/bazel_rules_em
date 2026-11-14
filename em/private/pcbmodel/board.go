// SPDX-License-Identifier: Apache-2.0

package pcbmodel

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Points on a full circle, and on each rounded corner. Fixed, so the same
// board gives the same polygons.
const (
	circlePoints = 32
	cornerPoints = 8
)

// Pt is a point in mm.
type Pt struct{ X, Y float64 }

// Shape is a copper polygon on one layer, of one net.
type Shape struct {
	Layer, Net string
	Points     []Pt
}

// Hole is a plated hole: a via or a through-hole pad's barrel.
type Hole struct {
	Net         string
	At          Pt
	Drill, Size float64
	Layers      []string // the copper layers it joins, top first
}

// Pad is a pad that a port can name, as "REF.PAD".
type Pad struct {
	Net    string
	At     Pt
	Layers []string
}

// Board is a board's copper, in KiCad's frame (mm, Y down).
type Board struct {
	Layers    []string // copper, top first
	Thickness float64  // the board's (general (thickness)), mm
	Outline   [4]float64
	Shapes    []Shape
	Holes     []Hole
	Pads      map[string]Pad
	Nets      map[string]bool
}

func rot(p Pt, deg float64) Pt {
	if deg == 0 {
		return p
	}
	// KiCad turns counter-clockwise on the screen, with Y down.
	t := deg * math.Pi / 180
	c, s := math.Cos(t), math.Sin(t)
	return Pt{p.X*c + p.Y*s, -p.X*s + p.Y*c}
}

func circle(c Pt, r float64) []Pt {
	pts := make([]Pt, circlePoints)
	for k := range pts {
		t := 2 * math.Pi * float64(k) / circlePoints
		pts[k] = Pt{c.X + r*math.Cos(t), c.Y + r*math.Sin(t)}
	}
	return pts
}

// capsule is the outline of segment a-b widened to radius r, round ends.
func capsule(a, b Pt, r float64) []Pt {
	t0 := math.Atan2(b.Y-a.Y, b.X-a.X)
	half := circlePoints / 2
	var pts []Pt
	for _, e := range []struct {
		at    Pt
		start float64
	}{{b, t0 - math.Pi/2}, {a, t0 + math.Pi/2}} {
		for k := 0; k <= half; k++ {
			t := e.start + math.Pi*float64(k)/float64(half)
			pts = append(pts, Pt{e.at.X + r*math.Cos(t), e.at.Y + r*math.Sin(t)})
		}
	}
	return pts
}

func roundedRect(w, h, r float64) []Pt {
	r = math.Max(0, math.Min(r, math.Min(w/2, h/2)))
	if r == 0 {
		return []Pt{{-w / 2, -h / 2}, {w / 2, -h / 2}, {w / 2, h / 2}, {-w / 2, h / 2}}
	}
	var pts []Pt
	for _, c := range []struct{ x, y, a0 float64 }{
		{w/2 - r, h/2 - r, 0}, {-w/2 + r, h/2 - r, 90}, {-w/2 + r, -h/2 + r, 180}, {w/2 - r, -h/2 + r, 270},
	} {
		for k := 0; k <= cornerPoints; k++ {
			t := (c.a0 + 90*float64(k)/cornerPoints) * math.Pi / 180
			pts = append(pts, Pt{c.x + r*math.Cos(t), c.y + r*math.Sin(t)})
		}
	}
	return pts
}

// arcPoints are points along the arc through a, m and b, ends included.
func arcPoints(a, m, b Pt) []Pt {
	d := 2 * (a.X*(m.Y-b.Y) + m.X*(b.Y-a.Y) + b.X*(a.Y-m.Y))
	if math.Abs(d) < 1e-12 {
		return []Pt{a, b}
	}
	sq := func(p Pt) float64 { return p.X*p.X + p.Y*p.Y }
	ux := (sq(a)*(m.Y-b.Y) + sq(m)*(b.Y-a.Y) + sq(b)*(a.Y-m.Y)) / d
	uy := (sq(a)*(b.X-m.X) + sq(m)*(a.X-b.X) + sq(b)*(m.X-a.X)) / d
	r := math.Hypot(a.X-ux, a.Y-uy)
	t0 := math.Atan2(a.Y-uy, a.X-ux)
	tm := math.Atan2(m.Y-uy, m.X-ux)
	t1 := math.Atan2(b.Y-uy, b.X-ux)
	ccw := func(x, y float64) float64 { return math.Mod(math.Mod(y-x, 2*math.Pi)+2*math.Pi, 2*math.Pi) }
	sweep := ccw(t0, t1)
	if ccw(t0, tm) > sweep {
		sweep -= 2 * math.Pi
	}
	n := int(math.Ceil(math.Abs(sweep) / (2 * math.Pi) * circlePoints))
	if n < 2 {
		n = 2
	}
	pts := []Pt{a}
	for k := 1; k < n; k++ {
		t := t0 + sweep*float64(k)/float64(n)
		pts = append(pts, Pt{ux + r*math.Cos(t), uy + r*math.Sin(t)})
	}
	return append(pts, b)
}

func xy(n *Node, name string) (Pt, error) {
	c := n.One(name)
	if c == nil {
		return Pt{}, fmt.Errorf("(%s ...) has no (%s x y)", n.Head(), name)
	}
	x, err := c.Float(0)
	if err != nil {
		return Pt{}, err
	}
	y, err := c.Float(1)
	return Pt{x, y}, err
}

func num(n *Node, name string, def float64) float64 {
	c := n.One(name)
	if c == nil {
		return def
	}
	v, err := c.Float(0)
	if err != nil {
		return def
	}
	return v
}

func str(n *Node, name string) string {
	return n.One(name).Arg(0).Text()
}

func ptsOf(n *Node) []Pt {
	var out []Pt
	for _, q := range n.One("pts").All("xy") {
		x, _ := q.Float(0)
		y, _ := q.Float(1)
		out = append(out, Pt{x, y})
	}
	return out
}

// padOutlines returns a pad's outlines at its origin, unturned. A custom
// pad has its anchor and each primitive.
func padOutlines(pad *Node) ([][]Pt, error) {
	shape := pad.Arg(2).Text()
	w, h := 0.0, 0.0
	if s := pad.One("size"); s != nil {
		w, _ = s.Float(0)
		h, _ = s.Float(1)
	}
	switch shape {
	case "circle":
		return [][]Pt{circle(Pt{}, w/2)}, nil
	case "rect":
		return [][]Pt{roundedRect(w, h, 0)}, nil
	case "oval":
		r := math.Min(w, h) / 2
		if w >= h {
			return [][]Pt{capsule(Pt{-(w/2 - r), 0}, Pt{w/2 - r, 0}, r)}, nil
		}
		return [][]Pt{capsule(Pt{0, -(h/2 - r)}, Pt{0, h/2 - r}, r)}, nil
	case "roundrect":
		return [][]Pt{roundedRect(w, h, num(pad, "roundrect_rratio", 0.25)*math.Min(w, h))}, nil
	case "custom":
		anchor := roundedRect(w, h, 0)
		if str(pad.One("options"), "anchor") == "circle" {
			anchor = circle(Pt{}, w/2)
		}
		out := [][]Pt{anchor}
		for _, p := range pad.One("primitives").Kids[1:] {
			switch p.Head() {
			case "gr_poly":
				out = append(out, ptsOf(p))
			case "gr_circle":
				c, err := xy(p, "center")
				if err != nil {
					return nil, err
				}
				e, err := xy(p, "end")
				if err != nil {
					return nil, err
				}
				out = append(out, circle(c, math.Hypot(e.X-c.X, e.Y-c.Y)))
			case "gr_rect":
				a, err := xy(p, "start")
				if err != nil {
					return nil, err
				}
				b, err := xy(p, "end")
				if err != nil {
					return nil, err
				}
				out = append(out, []Pt{{a.X, a.Y}, {b.X, a.Y}, {b.X, b.Y}, {a.X, b.Y}})
			default:
				return nil, fmt.Errorf("custom pad primitive %s: not read", p.Head())
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("pad shape %q: not read", shape)
}

func copperLayers(board *Node) []string {
	var names []string
	for _, l := range board.One("layers").Kids[1:] {
		if n := l.Arg(0).Text(); l.List && strings.HasSuffix(n, ".Cu") {
			names = append(names, n)
		}
	}
	key := func(n string) int {
		switch n {
		case "F.Cu":
			return -1
		case "B.Cu":
			return 1000
		}
		k, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(n, "In"), ".Cu"))
		return k
	}
	sort.Slice(names, func(i, j int) bool { return key(names[i]) < key(names[j]) })
	return names
}

func expand(names []string, copper []string) []string {
	want := map[string]bool{}
	for _, n := range names {
		switch n {
		case "*.Cu":
			for _, c := range copper {
				want[c] = true
			}
		case "F&B.Cu":
			want["F.Cu"], want["B.Cu"] = true, true
		default:
			want[n] = true
		}
	}
	var out []string
	for _, c := range copper {
		if want[c] {
			out = append(out, c)
		}
	}
	return out
}

func layerNames(n *Node) []string {
	var out []string
	if l := n.One("layers"); l != nil {
		for _, a := range l.Kids[1:] {
			out = append(out, a.Text())
		}
	}
	return out
}

// ReadBoard reads a .kicad_pcb (KiCad 7 to 9).
func ReadBoard(text string) (*Board, error) {
	root, err := Parse(text)
	if err != nil {
		return nil, err
	}
	if root.Head() != "kicad_pcb" {
		return nil, fmt.Errorf("not a KiCad board: it starts with (%s", root.Head())
	}
	b := &Board{Layers: copperLayers(root), Pads: map[string]Pad{}, Nets: map[string]bool{}}
	if len(b.Layers) < 2 {
		return nil, fmt.Errorf("the board has %d copper layers", len(b.Layers))
	}
	b.Thickness = num(root.One("general"), "thickness", 1.6)
	byNum := map[int]string{}
	for _, n := range root.All("net") {
		k, _ := n.Float(0)
		byNum[int(k)] = n.Arg(1).Text()
		if t := n.Arg(1).Text(); t != "" {
			b.Nets[t] = true
		}
	}
	netOf := func(n *Node) string {
		c := n.One("net")
		if c == nil {
			return ""
		}
		if a := c.Arg(1); a != nil {
			return a.Text()
		}
		if a := c.Arg(0); a != nil && a.IsNum {
			return byNum[int(a.Num)]
		}
		return c.Arg(0).Text() // a net by name
	}

	for _, s := range root.All("segment") {
		a, err := xy(s, "start")
		if err != nil {
			return nil, err
		}
		e, err := xy(s, "end")
		if err != nil {
			return nil, err
		}
		b.Shapes = append(b.Shapes, Shape{str(s, "layer"), netOf(s), capsule(a, e, num(s, "width", 0)/2)})
	}
	for _, s := range root.All("arc") {
		a, err := xy(s, "start")
		if err != nil {
			return nil, err
		}
		m, err := xy(s, "mid")
		if err != nil {
			return nil, err
		}
		e, err := xy(s, "end")
		if err != nil {
			return nil, err
		}
		pts := arcPoints(a, m, e)
		for k := 1; k < len(pts); k++ {
			b.Shapes = append(b.Shapes, Shape{str(s, "layer"), netOf(s), capsule(pts[k-1], pts[k], num(s, "width", 0)/2)})
		}
	}
	index := map[string]int{}
	for i, l := range b.Layers {
		index[l] = i
	}
	for _, v := range root.All("via") {
		at, err := xy(v, "at")
		if err != nil {
			return nil, err
		}
		ends := layerNames(v)
		if len(ends) < 2 {
			ends = []string{"F.Cu", "B.Cu"}
		}
		i0, ok0 := index[ends[0]]
		i1, ok1 := index[ends[len(ends)-1]]
		if !ok0 || !ok1 {
			return nil, fmt.Errorf("via at %v: layers %v are not copper layers", at, ends)
		}
		if i0 > i1 {
			i0, i1 = i1, i0
		}
		b.Holes = append(b.Holes, Hole{netOf(v), at, num(v, "drill", 0), num(v, "size", 0), b.Layers[i0 : i1+1]})
	}
	for _, z := range root.All("zone") {
		net := netOf(z)
		if net == "" {
			net = str(z, "net_name")
		}
		for _, fp := range z.All("filled_polygon") {
			b.Shapes = append(b.Shapes, Shape{str(fp, "layer"), net, ptsOf(fp)})
		}
	}

	for _, fp := range root.All("footprint") {
		at := fp.One("at")
		fx, _ := at.Float(0)
		fy, _ := at.Float(1)
		frot, _ := at.Float(2)
		ref := ""
		for _, p := range fp.All("property") {
			if p.Arg(0).Text() == "Reference" {
				ref = p.Arg(1).Text()
			}
		}
		for _, t := range fp.All("fp_text") { // KiCad 7
			if t.Arg(0).Text() == "reference" {
				ref = t.Arg(1).Text()
			}
		}
		for _, pad := range fp.All("pad") {
			pat := pad.One("at")
			px, _ := pat.Float(0)
			py, _ := pat.Float(1)
			// The pad's angle in the file already includes the footprint's.
			prot, _ := pat.Float(2)
			o := rot(Pt{px, py}, frot)
			c := Pt{fx + o.X, fy + o.Y}
			layers := expand(layerNames(pad), b.Layers)
			if len(layers) == 0 {
				continue
			}
			net := netOf(pad)
			if name := pad.Arg(0).Text(); name != "" {
				key := ref + "." + name
				if _, dup := b.Pads[key]; !dup {
					b.Pads[key] = Pad{net, c, layers}
				}
			}
			outlines, err := padOutlines(pad)
			if err != nil {
				return nil, fmt.Errorf("%s pad %s: %v", ref, pad.Arg(0).Text(), err)
			}
			for _, poly := range outlines {
				pts := make([]Pt, len(poly))
				for k, q := range poly {
					r := rot(q, prot)
					pts[k] = Pt{c.X + r.X, c.Y + r.Y}
				}
				for _, l := range layers {
					b.Shapes = append(b.Shapes, Shape{l, net, pts})
				}
			}
			if pad.Arg(1).Text() == "thru_hole" {
				if d := pad.One("drill"); d != nil {
					if dia, err := d.Float(0); err == nil {
						w, _ := pad.One("size").Float(0)
						b.Holes = append(b.Holes, Hole{net, c, dia, w, b.Layers})
					}
				}
			}
		}
	}

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	grow := func(p Pt) {
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
		minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
	}
	for _, kind := range []string{"gr_line", "gr_arc", "gr_rect", "gr_circle", "gr_poly"} {
		for _, g := range root.All(kind) {
			if str(g, "layer") != "Edge.Cuts" {
				continue
			}
			switch kind {
			case "gr_circle":
				c, _ := xy(g, "center")
				e, _ := xy(g, "end")
				r := math.Hypot(e.X-c.X, e.Y-c.Y)
				grow(Pt{c.X - r, c.Y - r})
				grow(Pt{c.X + r, c.Y + r})
			case "gr_poly":
				for _, p := range ptsOf(g) {
					grow(p)
				}
			case "gr_arc":
				a, _ := xy(g, "start")
				m, _ := xy(g, "mid")
				e, _ := xy(g, "end")
				for _, p := range arcPoints(a, m, e) {
					grow(p)
				}
			default:
				for _, name := range []string{"start", "end"} {
					if p, err := xy(g, name); err == nil {
						grow(p)
					}
				}
			}
		}
	}
	if math.IsInf(minX, 1) {
		return nil, fmt.Errorf("the board has no outline (Edge.Cuts)")
	}
	b.Outline = [4]float64{minX, minY, maxX, maxY}
	return b, nil
}
