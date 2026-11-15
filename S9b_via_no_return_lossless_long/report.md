# S9b_via_no_return_lossless_long

openEMS FDTD, 2 ports, each excited in turn; copper as sheets.

* Band: 0.01 to 10 GHz, 1000 points.
* Mesh: 935424 cells, at most 0.25 mm; 4 mm of air, its outer 8 cells an absorbing boundary (PML).
* Each run takes 60000 time steps.
* Ports: lumped, 50 ohm, from the pad to the nearest copper layer with copper under it.
* TDR: a step of 97 ps rise time (10 to 90 %), over the 3137 ps each run simulated.

| Port | Pad | Net | Layer |
|---|---|---|---|
| P1 | J1.1 | SIG | F.Cu |
| P2 | J2.1 | SIG | B.Cu |

## S-parameters (dB)

| | 0.01 GHz | 5.01 GHz | 10 GHz |
|---|---:|---:|---:|
| SP1,P1 | -1.38 | -11.09 | -30.60 |
| SP2,P1 | -16.11 | -0.12 | -0.05 |
| SP1,P2 | -16.11 | -0.12 | -0.05 |
| SP2,P2 | -1.38 | -11.09 | -30.60 |

## TDR impedance

| Port | min (ohm) | max (ohm) |
|---|---:|---:|
| P1 | 48.3 | 605.7 |
| P2 | 48.3 | 605.7 |

![S-parameters](s_params.png)

![Input impedance](impedance.png)

![TDR](tdr.png)
