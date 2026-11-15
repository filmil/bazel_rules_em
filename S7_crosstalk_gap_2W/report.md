# S7_crosstalk_gap_2W

openEMS FDTD, 4 ports, each excited in turn; copper as sheets.

* Band: 0.01 to 10 GHz, 1000 points.
* Mesh: 946176 cells, at most 0.25 mm; 4 mm of air, its outer 8 cells an absorbing boundary (PML).
* Each run takes 20000 time steps.
* Ports: lumped, 50 ohm, from the pad to the nearest copper layer with copper under it.
* TDR: a step of 97 ps rise time (10 to 90 %), over the 1074 ps each run simulated.

| Port | Pad | Net | Layer |
|---|---|---|---|
| P1 | J1.1 | AGG | F.Cu |
| P2 | J2.1 | AGG | F.Cu |
| P3 | J3.1 | VIC | F.Cu |
| P4 | J4.1 | VIC | F.Cu |

## S-parameters (dB)

| | 0.01 GHz | 5.01 GHz | 10 GHz |
|---|---:|---:|---:|
| SP1,P1 | -26.98 | -22.78 | -17.27 |
| SP2,P1 | -0.39 | -0.54 | -0.73 |
| SP3,P1 | -64.24 | -42.98 | -31.36 |
| SP4,P1 | -69.29 | -21.38 | -15.23 |
| SP1,P2 | -0.39 | -0.54 | -0.73 |
| SP2,P2 | -26.98 | -22.78 | -17.27 |
| SP3,P2 | -69.29 | -21.38 | -15.23 |
| SP4,P2 | -64.24 | -42.98 | -31.36 |
| SP1,P3 | -64.24 | -42.98 | -31.36 |
| SP2,P3 | -69.29 | -21.38 | -15.23 |
| SP3,P3 | -26.98 | -22.78 | -17.27 |
| SP4,P3 | -0.39 | -0.54 | -0.73 |
| SP1,P4 | -69.29 | -21.38 | -15.23 |
| SP2,P4 | -64.24 | -42.98 | -31.36 |
| SP3,P4 | -0.39 | -0.54 | -0.73 |
| SP4,P4 | -26.98 | -22.78 | -17.27 |

## TDR impedance

| Port | min (ohm) | max (ohm) |
|---|---:|---:|
| P1 | 45.7 | 57.8 |
| P2 | 45.7 | 57.8 |
| P3 | 45.7 | 57.8 |
| P4 | 45.7 | 57.8 |

![S-parameters](s_params.png)

![Input impedance](impedance.png)

![TDR](tdr.png)
