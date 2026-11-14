// SPDX-License-Identifier: Apache-2.0

// Package model_test reads what kicad_em_model writes for the fixture
// board.
package model_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var files = flag.String("files", "", "the model's outputs, as runfiles paths, space-separated")

// output returns the runfiles path of the output whose name ends in suffix.
func output(t *testing.T, suffix string) string {
	t.Helper()
	for _, f := range strings.Fields(*files) {
		if strings.HasSuffix(f, suffix) {
			return f
		}
	}
	t.Fatalf("no output *%s in %q", suffix, *files)
	return ""
}

func read(t *testing.T, rel string) []byte {
	t.Helper()
	p, err := runfiles.Rlocation(rel)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestJSON(t *testing.T) {
	var m struct {
		Nets   []string
		Ports  []struct{ Name, Pad, Net, Layer string }
		Copper []struct {
			Name      string
			Z         float64
			Thickness float64
		}
		Shapes []struct{ Layer, Net string }
	}
	if err := json.Unmarshal(read(t, output(t, ".json")), &m); err != nil {
		t.Fatal(err)
	}
	if strings.Join(m.Nets, ",") != "D+,D-,GND" {
		t.Errorf("nets %v", m.Nets)
	}
	var ports []string
	for _, p := range m.Ports {
		ports = append(ports, p.Name+"="+p.Pad+"/"+p.Net+"/"+p.Layer)
	}
	if got := strings.Join(ports, " "); got != "P1=J1.1/D+/F.Cu P2=J1.2/D-/F.Cu P3=R1.1/D-/F.Cu P4=R1.2/D+/F.Cu" {
		t.Errorf("ports %s", got)
	}
	if len(m.Copper) != 2 || m.Copper[0].Z != 1.545 {
		t.Errorf("copper %+v", m.Copper)
	}
	gndBottom := false
	for _, s := range m.Shapes {
		gndBottom = gndBottom || s.Net == "GND" && s.Layer == "B.Cu"
	}
	if !gndBottom {
		t.Error("no GND pour on B.Cu")
	}
}

func TestCSX(t *testing.T) {
	p, err := runfiles.Rlocation(output(t, ".csx"))
	if err != nil {
		t.Fatal(err)
	}
	xml, err := os.ReadFile(filepath.Join(p, "model.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`DeltaUnit="0.001"`,
		// The dielectric from the bottom copper's top to the top copper's
		// bottom, and the via's barrel through both.
		`Z="3.500000e-02"`, `Z="1.545000e+00"`, `Z="1.580000e+00"`,
		`<Material ID="0" Name="dielectric_1"`,
		`Epsilon="4.500000e+00,`,
		`Name="net:D+"`, `Name="net:D-"`, `Name="net:GND"`,
		"<LinPoly", "<Cylinder",
	} {
		if !bytes.Contains(xml, []byte(want)) {
			t.Errorf("model.xml has no %s", want)
		}
	}
}

func TestPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(read(t, output(t, ".png"))))
	if err != nil {
		t.Fatal(err)
	}
	if r := img.Bounds(); r.Dx() != 220 || r.Dy() != 120 {
		t.Errorf("picture %dx%d, want 220x120", r.Dx(), r.Dy())
	}
}
