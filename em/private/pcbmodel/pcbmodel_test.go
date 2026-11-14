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
		"R1.1": {"D+", Pt{118, 105.5}, []string{"F.Cu"}},
		"R1.2": {"D-", Pt{118, 104.5}, []string{"F.Cu"}},
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
	// The custom pad: its circle anchor and its square, turned with R1.
	n := 0
	for _, s := range b.Shapes {
		if s.Net == "D-" && len(s.Points) == 4 {
			n++
			if math.Abs(area(s.Points)-0.16) > 1e-9 && math.Abs(area(s.Points)-0.4) > 1e-9 {
				t.Errorf("a D- quadrilateral of area %v", area(s.Points))
			}
		}
	}
	if n != 2 { // J1.2's rect and R1.2's square
		t.Errorf("%d D- quadrilaterals, want 2", n)
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
