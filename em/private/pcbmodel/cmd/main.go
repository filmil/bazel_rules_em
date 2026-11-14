// SPDX-License-Identifier: Apache-2.0

// Command pcbmodel is kicad_em_model's board reader: it writes the model
// of a board's nets as JSON, and a picture of it.
//
//	pcbmodel --pcb B.kicad_pcb --net USB_DP --net USB_DM \
//	  --port P1=J1.A6 --copper 0.035 --dielectric "1.51mm 4.5 0.02" \
//	  --json model.json --png model.png
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/filmil/bazel_rules_em/em/private/pcbmodel"
)

type list []string

func (l *list) String() string     { return strings.Join(*l, ",") }
func (l *list) Set(s string) error { *l = append(*l, s); return nil }

func run(args []string) error {
	fs := flag.NewFlagSet("pcbmodel", flag.ContinueOnError)
	var nets, ports, dielectrics list
	pcb := fs.String("pcb", "", "the board, a .kicad_pcb")
	fs.Var(&nets, "net", "a net to model (repeat)")
	fs.Var(&ports, "port", "a port, NAME=REF.PAD (repeat)")
	copper := fs.String("copper", "", "each copper layer's thickness, in mm")
	fs.Var(&dielectrics, "dielectric", `a dielectric, top first, "<thickness>mm <epsilon_r> <loss tangent>" (repeat)`)
	px := fs.Int("px_per_mm", 20, "the picture's pixels per mm")
	jsonOut := fs.String("json", "", "the model to write")
	pngOut := fs.String("png", "", "the picture to write")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pcb == "" || *jsonOut == "" || *pngOut == "" {
		return fmt.Errorf("--pcb, --json and --png are needed")
	}
	o := pcbmodel.Options{Nets: nets, Ports: map[string]string{}}
	var err error
	if o.Copper, err = pcbmodel.ParseMM(*copper); err != nil {
		return fmt.Errorf("--copper: %v", err)
	}
	for _, d := range dielectrics {
		v, err := pcbmodel.ParseDielectric(d)
		if err != nil {
			return err
		}
		o.Dielectrics = append(o.Dielectrics, v)
	}
	for _, p := range ports {
		name, pad, ok := strings.Cut(p, "=")
		if !ok || name == "" || pad == "" {
			return fmt.Errorf("--port %q: want NAME=REF.PAD", p)
		}
		if _, dup := o.Ports[name]; dup {
			return fmt.Errorf("--port %q: a second port named %s", p, name)
		}
		o.Ports[name] = pad
	}
	text, err := os.ReadFile(*pcb)
	if err != nil {
		return err
	}
	b, err := pcbmodel.ReadBoard(string(text))
	if err != nil {
		return fmt.Errorf("%s: %v", *pcb, err)
	}
	m, err := pcbmodel.Build(b, o)
	if err != nil {
		return fmt.Errorf("%s: %v", *pcb, err)
	}
	pic, err := m.Picture(*px)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*jsonOut, m.JSON(), 0o644); err != nil {
		return err
	}
	return os.WriteFile(*pngOut, pic, 0o644)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "kicad_em_model:", err)
		os.Exit(1)
	}
}
