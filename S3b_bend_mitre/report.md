# S3b_bend_mitre

openEMS FDTD, 2 ports, each excited in turn; copper as sheets.

* Band: 0.01 to 10 GHz, 1000 points.
* Mesh: 731808 cells, at most 0.25 mm; 4 mm of air, its outer 8 cells an absorbing boundary (PML).
* Each run takes 20000 time steps.
* Ports: lumped, 50 ohm, from the pad to the nearest copper layer with copper under it.
* TDR: a step of 97 ps rise time (10 to 90 %), over the 4618 ps each run simulated.

| Port | Pad | Net | Layer |
|---|---|---|---|
| P1 | J1.1 | SIG | F.Cu |
| P2 | J2.1 | SIG | F.Cu |

## S-parameters (dB)

| | 0.01 GHz | 5.01 GHz | 10 GHz |
|---|---:|---:|---:|
| SP1,P1 | -26.08 | -31.45 | -19.32 |
| SP2,P1 | -0.46 | -0.96 | -2.75 |
| SP1,P2 | -0.46 | -0.96 | -2.79 |
| SP2,P2 | -26.08 | -26.47 | -12.24 |

## TDR impedance

| Port | min (ohm) | max (ohm) |
|---|---:|---:|
| P1 | 44.1 | 52.7 |
| P2 | 44.6 | 51.8 |

![S-parameters](s_params.png)

![Input impedance](impedance.png)

![TDR](tdr.png)
