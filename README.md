# pcb-em-intuition: simulated examples

openEMS FDTD results for the Maker tutorial "EM intuition for PCB routing"
(Archimedes), made with hdlf_rules_em's kicad_em_model and kicad_em_sim
(https://github.com/filmil/bazel_rules_em). This branch is data, never
merged.

Each directory is one case:

* `board.kicad_pcb`: the board, written by `gen.py`;
* `spec.json`: its nets, ports, dielectrics and a one-line description;
* `model.json`, `model.png`: the model kicad_em_model reads from it;
* `<case>.s<N>p`: S-parameters, Touchstone 1.1, Hz S RI R 50, 10 MHz to
  10 GHz in steps of 10 MHz (the grid starts at its step: IFFT-ready);
* `tdr_<port>.csv`: `time_ns,impedance_ohm`, from S11 with a Hann-windowed
  step of about 97 ps rise (10-90 %);
* `report.md`, `s_params.png`, `impedance.png`, `tdr.png`.

Setup, all cases:

* 2-layer: F.Cu / 1.53 mm FR-4 / B.Cu. 4-layer: F.Cu / 0.2 mm / In1 GND /
  1.06 mm / In2 GND / 0.2 mm / B.Cu. All epsilon_r 4.3, tan d 0.02, 35 um
  copper (drawn as sheets of no thickness).
* Ports: lumped, 50 ohm, 0.2 mm wide, from the end pad to the reference
  plane.
* Mesh at most 0.25 mm; 4 mm of air, its outer 8 cells PML; 20000 time
  steps a run; openEMS multithreaded, 4 threads.
* Dielectric loss is a conductivity matched to tan d at 5 GHz, so loss is
  overstated at low frequency (S1's S21 is -0.48 dB at 10 MHz) and about
  right near the middle of the band.
