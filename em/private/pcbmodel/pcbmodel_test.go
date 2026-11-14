// SPDX-License-Identifier: Apache-2.0

package pcbmodel

import (
	"bytes"
	"encoding/json"
	stdcolor "image/color"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
)

func board(t *testing.T) *Board {
	t.Helper()
	text, err := os.ReadFile("testdata/pair.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ReadBoard(string(text))
	if err != nil {
		t.Fatalf("ReadBoard: %v", err)
	}
	return b
}

func area(pts []Pt) float64 {
	a := 0.0
	for k := range pts {
		p, q := pts[k], pts[(k+1)%len(pts)]
		a += p.X*q.Y - q.X*p.Y
	}
	return math.Abs(a) / 2
}

func close(a, b Pt) bool { return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 }

func TestParse(t *testing.T) {
	n, err := Parse(`(a (b 1.5 "x \"y\"") sym)`)
	if err != nil {
		t.Fatal(err)
	}
	if n.Head() != "a" || n.One("b").Arg(1).Text() != `x "y"` || n.Arg(1).Text() != "sym" {
		t.Errorf("Parse: %+v", n)
	}
	if v, _ := n.One("b").Float(0); v != 1.5 {
		t.Errorf("number %v", v)
	}
	for _, bad := range []string{"(a", "a)", `(a "b`, "(a))"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q): no error", bad)
		}
	}
}

func TestReadBoard(t *testing.T) {
	b := board(t)
	if strings.Join(b.Layers, ",") != "F.Cu,B.Cu" || b.Thickness != 1.6 {
		t.Errorf("layers %v, thickness %v", b.Layers, b.Thickness)
	}
	if b.Outline != [4]float64{100, 100, 120, 110} {
		t.Errorf("outline %v", b.Outline)
	}
	for ref, want := range map[string]Pad{
		"J1.1": {"D+", Pt{102, 104.5}, []string{"F.Cu"}},
		"J1.3": {"GND", Pt{100, 105}, []string{"F.Cu", "B.Cu"}},
		// R1 is turned 90°: its pad 1 at (-0.5, 0) goes to +Y (down).
		"R1.1": {"D-", Pt{118, 105.5}, []string{"F.Cu"}},
		"R1.2": {"D+", Pt{118, 104.5}, []string{"F.Cu"}},
	} {
		got, ok := b.Pads[ref]
		if !ok || got.Net != want.Net || !close(got.At, want.At) || strings.Join(got.Layers, ",") != strings.Join(want.Layers, ",") {
			t.Errorf("pad %s: %+v, want %+v", ref, got, want)
		}
	}
	if len(b.Holes) != 2 {
		t.Errorf("%d holes, want the via and J1.3's barrel", len(b.Holes))
	}
	// The roundrect pad: 1 x 0.4, radius 0.1.
	var pad []Pt
	for _, s := range b.Shapes {
		if s.Net == "D+" && s.Layer == "F.Cu" && len(s.Points) == 4*(cornerPoints+1) {
			pad = s.Points
		}
	}
	want := 0.4 - (4-math.Pi)*0.01
	if math.Abs(area(pad)-want) > 0.002 {
		t.Errorf("roundrect pad area %v, want %v", area(pad), want)
	}
	// J1.2's rect, and the custom pad R1.2's square (turned with R1).
	n := 0
	for _, s := range b.Shapes {
		if (s.Net == "D-" || s.Net == "D+") && len(s.Points) == 4 {
			n++
			if math.Abs(area(s.Points)-0.16) > 1e-9 && math.Abs(area(s.Points)-0.4) > 1e-9 {
				t.Errorf("a %s quadrilateral of area %v", s.Net, area(s.Points))
			}
		}
	}
	if n != 2 {
		t.Errorf("%d quadrilaterals, want 2", n)
	}
}

func options() Options {
	return Options{
		Nets:        []string{"D-", "D+", "GND"},
		Ports:       map[string]string{"P1": "J1.1", "P2": "J1.2", "P3": "R1.1", "P4": "R1.2"},
		Copper:      0.035,
		Dielectrics: []Dielectric{{1.51, 4.5, 0.02}},
	}
}

func TestBuild(t *testing.T) {
	m, err := Build(board(t), options())
	if err != nil {
		t.Fatal(err)
	}
	var j struct {
		Board       map[string]float64
		Copper      []map[string]any
		Dielectrics []map[string]any
		Nets        []string
		Ports       []map[string]any
		Holes       []map[string]any
		Shapes      []map[string]any
	}
	if err := json.Unmarshal(m.JSON(), &j); err != nil {
		t.Fatalf("model.json: %v\n%s", err, m.JSON())
	}
	if j.Board["min_y"] != -110 || j.Board["max_y"] != -100 || j.Board["thickness"] != 1.6 {
		t.Errorf("board %v: Y is not up", j.Board)
	}
	if j.Copper[0]["name"] != "F.Cu" || j.Copper[0]["z"] != 1.545 || j.Copper[1]["z"] != 0.0 {
		t.Errorf("copper %v", j.Copper)
	}
	if j.Dielectrics[0]["z"] != 0.035 || j.Dielectrics[0]["epsilon_r"] != 4.5 {
		t.Errorf("dielectrics %v", j.Dielectrics)
	}
	if strings.Join(j.Nets, ",") != "D+,D-,GND" {
		t.Errorf("nets %v", j.Nets)
	}
	if len(j.Ports) != 4 || j.Ports[0]["name"] != "P1" || j.Ports[0]["y"] != -104.5 || j.Ports[0]["layer"] != "F.Cu" {
		t.Errorf("ports %v", j.Ports)
	}
	if len(j.Holes) != 2 {
		t.Errorf("holes %v", j.Holes)
	}
	if !bytes.Equal(m.JSON(), m.JSON()) {
		t.Error("JSON differs from run to run")
	}
	// Only the chosen nets.
	o := options()
	o.Nets = []string{"D+"}
	o.Ports = map[string]string{"P1": "J1.1"}
	m2, err := Build(board(t), o)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range m2.Shapes {
		if s.Net != "D+" {
			t.Errorf("a %s shape in a D+ model", s.Net)
		}
	}
	if len(m2.Holes) != 0 {
		t.Errorf("GND's holes in a D+ model")
	}
}

func TestBuildErrors(t *testing.T) {
	b := board(t)
	for what, change := range map[string]func(*Options){
		"unknown net":      func(o *Options) { o.Nets = append(o.Nets, "VBUS") },
		"no nets":          func(o *Options) { o.Nets = nil },
		"dielectrics":      func(o *Options) { o.Dielectrics = append(o.Dielectrics, o.Dielectrics[0]) },
		"copper":           func(o *Options) { o.Copper = 0 },
		"unknown pad":      func(o *Options) { o.Ports["P5"] = "J9.1" },
		"pad of other net": func(o *Options) { o.Nets = []string{"D+", "D-"}; o.Ports["P5"] = "J1.3" },
	} {
		o := options()
		change(&o)
		if _, err := Build(b, o); err == nil {
			t.Errorf("%s: no error", what)
		}
	}
	for _, bad := range []string{"1.51 4.5 0.02", "0mm 4.5 0.02", "1mm 0.5 0", "1mm 4.5"} {
		if _, err := ParseDielectric(bad); err == nil {
			t.Errorf("ParseDielectric(%q): no error", bad)
		}
	}
	if _, err := ReadBoard("(kicad_pcb (layers (0 \"F.Cu\" signal) (2 \"B.Cu\" signal)))"); err == nil {
		t.Error("a board with no outline: no error")
	}
	if _, err := ReadBoard("(footprint x)"); err == nil {
		t.Error("not a board: no error")
	}
}

func TestPicture(t *testing.T) {
	m, err := Build(board(t), options())
	if err != nil {
		t.Fatal(err)
	}
	a, err := m.Picture(10)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := m.Picture(10)
	if !bytes.Equal(a, b) {
		t.Error("the picture differs from run to run")
	}
	img, err := png.Decode(bytes.NewReader(a))
	if err != nil {
		t.Fatal(err)
	}
	// 20 x 10 mm and 1 mm around, at 10 pixels per mm.
	if r := img.Bounds(); r.Dx() != 220 || r.Dy() != 120 {
		t.Errorf("picture %dx%d, want 220x120", r.Dx(), r.Dy())
	}
	// The middle of the D- track, (110, 105.5) in KiCad: top copper over
	// the bottom's GND pour, so purple.
	x, y := (110-99)*10, int((105.5-99)*10)
	if c := rgba(img.At(x, y)); c != both {
		t.Errorf("pixel at the D- track is %v, want %v", c, both)
	}
}

func rgba(c stdcolor.Color) stdcolor.NRGBA { return stdcolor.NRGBAModel.Convert(c).(stdcolor.NRGBA) }

// A four-layer board: inner layers, a footprint on the bottom, a blind
// via, an oval drill with an offset pad, two pads of one name, an
// Edge.Cuts line inside a footprint, and an unfilled zone.
const fourLayers = `(kicad_pcb (version 20241229)
	(general (thickness 1.6))
	(layers (0 "F.Cu" signal) (4 "In1.Cu" signal) (6 "In2.Cu" signal) (2 "B.Cu" signal) (25 "Edge.Cuts" user))
	(net 0 "") (net 1 "GND") (net 2 "S") (net 3 "VBUS")
	(footprint "J" (layer "B.Cu") (at 10 10)
		(property "Reference" "J2")
		(pad "1" smd rect (at 0 0) (size 1 1) (layers "B.Cu") (net 2 "S"))
		(pad "S" thru_hole oval (at 3 0) (size 2 1.2) (drill oval 1.6 0.8 (offset 0.1 0)) (layers "*.Cu") (net 1 "GND"))
		(pad "S" thru_hole oval (at -3 0) (size 2 1.2) (drill oval 1.6 0.8) (layers "*.Cu") (net 1 "GND"))
		(fp_line (start 0 5) (end 0 25) (layer "Edge.Cuts")))
	(via blind (at 5 5) (size 0.5) (drill 0.2) (layers "F.Cu" "In1.Cu") (net 2))
	(segment (start 0 0) (end 5 5) (width 0.2) (layer "In2.Cu") (net 2))
	(zone (net 3) (net_name "VBUS") (layer "In1.Cu"))
	(gr_line (start 0 0) (end 20 0) (layer "Edge.Cuts"))
	(gr_line (start 20 0) (end 20 20) (layer "Edge.Cuts")))`

func TestFourLayers(t *testing.T) {
	b, err := ReadBoard(fourLayers)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(b.Layers, ",") != "F.Cu,In1.Cu,In2.Cu,B.Cu" {
		t.Errorf("layers %v", b.Layers)
	}
	// The footprint's Edge.Cuts line reaches Y 35.
	if b.Outline != [4]float64{0, 0, 20, 35} {
		t.Errorf("outline %v", b.Outline)
	}
	if p := b.Pads["J2.1"]; strings.Join(p.Layers, ",") != "B.Cu" {
		t.Errorf("J2.1 on %v, want B.Cu", p.Layers)
	}
	if b.Shared["J2.S"] != 2 {
		t.Errorf("J2.S shared by %d pads, want 2", b.Shared["J2.S"])
	}
	var blind, slot *Hole
	for i := range b.Holes {
		h := &b.Holes[i]
		if h.Net == "S" {
			blind = h
		}
		if h.Slot != nil && slot == nil {
			slot = h
		}
	}
	if blind == nil || strings.Join(blind.Layers, ",") != "F.Cu,In1.Cu" {
		t.Errorf("blind via %+v", blind)
	}
	// A 1.6 x 0.8 slot: a 0.8 x 0.8 square and a 0.8 circle.
	if slot == nil || math.Abs(area(slot.Slot)-(0.64+math.Pi*0.16)) > 0.01 || slot.Drill != 0.8 {
		t.Errorf("slot %+v", slot)
	}
	// The offset pad's copper is 0.1 mm right of its hole at (13, 10).
	found := false
	for _, s := range b.Shapes {
		if s.Net == "GND" && s.Layer == "In2.Cu" {
			minX := math.Inf(1)
			for _, p := range s.Points {
				minX = math.Min(minX, p.X)
			}
			found = found || math.Abs(minX-12.1) < 1e-9
		}
	}
	if !found {
		t.Error("no GND pad on In2.Cu starting at X 12.1 (13 - 1 + 0.1)")
	}
	o := Options{Nets: []string{"S", "GND"}, Ports: map[string]string{"P1": "J2.1"}, Copper: 0.035,
		Dielectrics: []Dielectric{{0.2, 4.4, 0.02}, {1.0, 4.5, 0.02}, {0.2, 4.4, 0.02}}}
	m, err := Build(b, o)
	if err != nil {
		t.Fatal(err)
	}
	// From the bottom: B.Cu 0, d3 0.035, In2 0.235, d2 0.27, In1 1.27,
	// d1 1.305, F.Cu 1.505.
	var zs []string
	for _, l := range m.Copper {
		zs = append(zs, f(l.Z))
	}
	if got := strings.Join(zs, " "); got != "1.505000 1.270000 0.235000 0" {
		t.Errorf("copper Z %s", got)
	}
	if m.Dielectrics[1].Z != 0.27 {
		t.Errorf("the middle dielectric at Z %v, want 0.27", m.Dielectrics[1].Z)
	}
	if !strings.Contains(string(m.JSON()), `"slot": [`) {
		t.Error("model.json has no slot")
	}
	pic, err := m.Picture(5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(pic)); err != nil {
		t.Error(err)
	}
	for what, change := range map[string]func(*Options){
		"a shared pad name": func(o *Options) { o.Ports["P2"] = "J2.S" },
		"an unfilled zone":  func(o *Options) { o.Nets = append(o.Nets, "VBUS") },
		"a port's layer":    func(o *Options) { o.Ports["P1"] = "J2.1@F.Cu" },
		"a net twice":       func(o *Options) { o.Nets = append(o.Nets, "S") },
	} {
		o2 := o
		o2.Nets = append([]string(nil), o.Nets...)
		o2.Ports = map[string]string{"P1": "J2.1"}
		change(&o2)
		if _, err := Build(b, o2); err == nil {
			t.Errorf("%s: no error", what)
		}
	}
}

func TestNumbers(t *testing.T) {
	for _, bad := range []string{"NaN", "Inf", "-1", "0", "x", "0..1"} {
		if _, err := ParseMM(bad); err == nil {
			t.Errorf("ParseMM(%q): no error", bad)
		}
	}
	if v, err := ParseMM("0.035mm"); err != nil || v != 0.035 {
		t.Errorf("ParseMM(0.035mm) = %v, %v", v, err)
	}
	if d, err := ParseDielectric("0.2mm 4.4 2.5e-4"); err != nil || d.LossTangent != 2.5e-4 {
		t.Errorf("ParseDielectric with 2.5e-4: %+v, %v", d, err)
	}
	for _, bad := range []string{"1.51mm 4.5 0..02", "1.51mm NaN 0.02", "1.51mm 4.5 -1"} {
		if _, err := ParseDielectric(bad); err == nil {
			t.Errorf("ParseDielectric(%q): no error", bad)
		}
	}
	m, err := Build(board(t), options())
	if err != nil {
		t.Fatal(err)
	}
	for _, px := range []int{0, 101} {
		if _, err := m.Picture(px); err == nil {
			t.Errorf("Picture(%d): no error", px)
		}
	}
}
