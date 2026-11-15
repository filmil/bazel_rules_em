# S7_crosstalk_gap_1W

openEMS FDTD, 4 ports, each excited in turn; copper as sheets.

* Band: 0.01 to 10 GHz, 1000 points.
* Mesh: 924672 cells, at most 0.25 mm; 4 mm of air, its outer 8 cells an absorbing boundary (PML).
* Each run takes 20000 time steps.
* Ports: lumped, 50 ohm, from the pad to the nearest copper layer with copper under it.
* TDR: a step of 97 ps rise time (10 to 90 %), over the 986 ps each run simulated.

| Port | Pad | Net | Layer |
|---|---|---|---|
| P1 | J1.1 | AGG | F.Cu |
| P2 | J2.1 | AGG | F.Cu |
| P3 | J3.1 | VIC | F.Cu |
| P4 | J4.1 | VIC | F.Cu |

## S-parameters (dB)

| | 0.01 GHz | 5.01 GHz | 10 GHz |
|---|---:|---:|---:|
| SP1,P1 | -28.43 | -18.41 | -16.29 |
| SP2,P1 | -0.36 | -0.69 | -1.37 |
| SP3,P1 | -49.46 | -27.70 | -18.31 |
| SP4,P1 | -63.13 | -14.71 | -8.60 |
| SP1,P2 | -0.36 | -0.69 | -1.37 |
| SP2,P2 | -28.43 | -18.41 | -16.29 |
| SP3,P2 | -63.13 | -14.71 | -8.60 |
| SP4,P2 | -49.46 | -27.70 | -18.31 |
| SP1,P3 | -49.46 | -27.70 | -18.31 |
| SP2,P3 | -63.13 | -14.71 | -8.60 |
| SP3,P3 | -28.43 | -18.41 | -16.29 |
| SP4,P3 | -0.36 | -0.69 | -1.37 |
| SP1,P4 | -63.13 | -14.71 | -8.60 |
| SP2,P4 | -49.46 | -27.70 | -18.31 |
| SP3,P4 | -0.36 | -0.69 | -1.37 |
| SP4,P4 | -28.43 | -18.41 | -16.29 |

## TDR impedance

| Port | min (ohm) | max (ohm) |
|---|---:|---:|
| P1 | 46.4 | 58.9 |
| P2 | 46.4 | 58.9 |
| P3 | 46.4 | 58.9 |
| P4 | 46.4 | 58.9 |

![S-parameters](s_params.png)

![Input impedance](impedance.png)

![TDR](tdr.png)
