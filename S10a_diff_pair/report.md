# S10a_diff_pair

openEMS FDTD, 4 ports, each excited in turn; copper as sheets.

* Band: 0.01 to 10 GHz, 1000 points.
* Mesh: 924672 cells, at most 0.25 mm; 4 mm of air, its outer 8 cells an absorbing boundary (PML).
* Each run takes 20000 time steps.
* Ports: lumped, 50 ohm, from the pad to the nearest copper layer with copper under it.
* TDR: a step of 97 ps rise time (10 to 90 %), over the 1074 ps each run simulated.

| Port | Pad | Net | Layer |
|---|---|---|---|
| P1 | J1.1 | DP | F.Cu |
| P2 | J2.1 | DP | F.Cu |
| P3 | J3.1 | DN | F.Cu |
| P4 | J4.1 | DN | F.Cu |

## S-parameters (dB)

| | 0.01 GHz | 5.01 GHz | 10 GHz |
|---|---:|---:|---:|
| SP1,P1 | -27.43 | -19.12 | -16.24 |
| SP2,P1 | -0.35 | -0.82 | -2.04 |
| SP3,P1 | -49.85 | -18.80 | -13.37 |
| SP4,P1 | -47.46 | -12.88 | -6.82 |
| SP1,P2 | -0.35 | -0.82 | -2.04 |
| SP2,P2 | -27.43 | -19.12 | -16.24 |
| SP3,P2 | -47.46 | -12.88 | -6.82 |
| SP4,P2 | -49.85 | -18.80 | -13.37 |
| SP1,P3 | -49.85 | -18.80 | -13.37 |
| SP2,P3 | -47.46 | -12.88 | -6.82 |
| SP3,P3 | -27.43 | -19.12 | -16.24 |
| SP4,P3 | -0.35 | -0.82 | -2.04 |
| SP1,P4 | -47.46 | -12.88 | -6.82 |
| SP2,P4 | -49.85 | -18.80 | -13.37 |
| SP3,P4 | -0.35 | -0.82 | -2.04 |
| SP4,P4 | -27.43 | -19.12 | -16.24 |

## TDR impedance

| Port | min (ohm) | max (ohm) |
|---|---:|---:|
| P1 | 46.0 | 60.1 |
| P2 | 46.0 | 60.1 |
| P3 | 46.0 | 60.1 |
| P4 | 46.0 | 60.1 |

![S-parameters](s_params.png)

![Input impedance](impedance.png)

![TDR](tdr.png)
