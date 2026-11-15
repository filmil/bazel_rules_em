# S8_microstrip_4layer_0.36mm_mesh0.125

openEMS FDTD, 2 ports, each excited in turn; copper as sheets.

* Band: 0.01 to 10 GHz, 1000 points.
* Mesh: 2462720 cells, at most 0.125 mm; 2 mm of air, its outer 8 cells an absorbing boundary (PML).
* Each run takes 20000 time steps.
* Ports: lumped, 50 ohm, from the pad to the nearest copper layer with copper under it.
* TDR: a step of 97 ps rise time (10 to 90 %), over the 1087 ps each run simulated.

| Port | Pad | Net | Layer |
|---|---|---|---|
| P1 | J1.1 | SIG | F.Cu |
| P2 | J2.1 | SIG | F.Cu |

## S-parameters (dB)

| | 0.01 GHz | 5.01 GHz | 10 GHz |
|---|---:|---:|---:|
| SP1,P1 | -26.28 | -28.36 | -26.25 |
| SP2,P1 | -0.44 | -0.46 | -0.47 |
| SP1,P2 | -0.44 | -0.46 | -0.47 |
| SP2,P2 | -26.28 | -28.36 | -26.25 |

## TDR impedance

| Port | min (ohm) | max (ohm) |
|---|---:|---:|
| P1 | 44.8 | 50.0 |
| P2 | 44.8 | 50.0 |

![S-parameters](s_params.png)

![Input impedance](impedance.png)

![TDR](tdr.png)
