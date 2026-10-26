# UPX 5.2.1 Binary Compression Performance and Size Analysis

This document details the binary size reduction and startup latency benchmarks for Kaniko binaries compressed using **UPX 5.2.1** (`--best` compression mode) compared to standard uncompressed binaries.

---

## 1. Overview

UPX (Ultimate Packer for eXecutables) compresses executable binaries, reducing image layer sizes and disk usage at the expense of a small one-time startup decompression latency during process launch.

The optional build argument `UPX_COMPRESS=true` in `deploy/Dockerfile` compresses non-debug Kaniko binaries (`executor`, `warmer`, and `executor-slim`) using UPX 5.2.1 across supported target platforms (`amd64`, `arm64`, `arm`, `386`, `ppc64le`, `riscv64`).

---

## 2. Binary Size Comparison

*Environment: Linux x86_64 (`linux/amd64`), Go 1.27.3, UPX 5.2.1 (`--best`)*

| Binary Component | Uncompressed Size | UPX 5.2.1 Size | Compression Ratio | Size Reduction |
| :--- | :---: | :---: | :---: | :---: |
| `out/executor` | **37.8 MB** (37,761,148 B) | **11.2 MB** (11,158,876 B) | 29.55% | **-70.45% (-26.6 MB)** |
| `out/warmer` | **17.4 MB** (17,395,836 B) | **5.8 MB** (5,763,400 B) | 33.13% | **-66.87% (-11.6 MB)** |
| `out/executor-slim` | **12.9 MB** (12,886,140 B) | **4.6 MB** (4,638,936 B) | 36.00% | **-64.00% (-8.2 MB)** |
| **Total** | **68.0 MB** | **21.6 MB** | **31.69%** | **-68.31% (-46.4 MB)** |

### Key Findings (Size)
* UPX 5.2.1 achieves an average **68.3% reduction** in uncompressed binary size across all non-debug components.
* For the main `kaniko-executor` container image, the binary size drops from ~37.8 MB to ~11.2 MB, saving **26.6 MB** of uncompressed layer storage.

---

## 3. Startup Performance Benchmarks

*Environment: Intel/AMD 4 vCPU, 8 GB RAM, Ubuntu 24.04 (Linux 6.8), 20 iterations per benchmark (`--help` flag).*

| Binary Component | Metric | Uncompressed Binary | UPX 5.2.1 Binary | Decompression Overhead |
| :--- | :--- | :---: | :---: | :---: |
| **`executor`** | Average Execution Time | **40.21 ms** | **224.02 ms** | **+183.81 ms** |
| | Min Execution Time | 36.25 ms | 219.72 ms | +183.47 ms |
| | Max Execution Time | 84.88 ms | 240.22 ms | +155.34 ms |
| **`warmer`** | Average Execution Time | **33.05 ms** | **128.01 ms** | **+94.96 ms** |
| | Min Execution Time | 31.26 ms | 124.62 ms | +93.36 ms |
| | Max Execution Time | 35.27 ms | 151.72 ms | +116.45 ms |
| **`executor-slim`** | Average Execution Time | **27.30 ms** | **103.32 ms** | **+76.03 ms** |
| | Min Execution Time | 26.40 ms | 100.89 ms | +74.49 ms |
| | Max Execution Time | 28.47 ms | 109.08 ms | +80.61 ms |

---

## 4. Trade-off Analysis & Recommendation

### Trade-offs

1. **Storage & Download Benefits**:
   * Saves ~46.4 MB of uncompressed binary footprint across executor and warmer binaries.
   * Significantly reduces container pull sizes over slow network connections or bandwidth-constrained edge environments.

2. **Runtime Overhead**:
   * Adds **~75–185 ms** of initial startup CPU overhead per process execution due to in-memory decompression upon binary invocation.
   * For container image builds that run for tens of seconds or minutes, a ~180 ms startup overhead represents an insignificant fraction (<0.5%) of the total job runtime.

### Recommendation
* Keep `UPX_COMPRESS=false` as the default setting to preserve immediate startup execution and lower CPU overhead for standard builds.
* Enable `UPX_COMPRESS=true` for CI/CD environments where container image layer size and network pull bandwidth are prioritized over a ~180ms startup delay.
