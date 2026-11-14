# S6a_cpwg_corner_vias

openEMS FDTD, 2 ports, each excited in turn; copper as sheets.

* Band: 0.01 to 10 GHz, 1000 points.
* Mesh: 880152 cells, at most 0.25 mm; 4 mm of air, its outer 8 cells an absorbing boundary (PML).
* Each run takes 20000 time steps.
* Ports: lumped, 50 ohm, from the pad to the nearest copper layer with copper under it.
* TDR: a step of 97 ps rise time (10 to 90 %), over the 4576 ps each run simulated.

| Port | Pad | Net | Layer |
|---|---|---|---|
| P1 | J1.1 | SIG | F.Cu |
| P2 | J2.1 | SIG | F.Cu |

## S-parameters (dB)

| | 0.01 GHz | 5.01 GHz | 10 GHz |
|---|---:|---:|---:|
| SP1,P1 | -24.31 | -7.52 | -2.95 |
| SP2,P1 | -0.56 | -1.48 | -4.28 |
| SP1,P2 | -0.56 | -1.49 | -4.29 |
| SP2,P2 | -24.32 | -7.51 | -2.94 |

## TDR impedance

| Port | min (ohm) | max (ohm) |
|---|---:|---:|
| P1 | 32.2 | 51.9 |
| P2 | 32.2 | 52.0 |

![S-parameters](s_params.png)

![Input impedance](impedance.png)

![TDR](tdr.png)
