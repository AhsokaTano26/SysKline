# MVP verification — 2026-11-02

Environment: Go 1.26.2, macOS ARM64 host, Linux containers on Docker Desktop.
The initial checks below were recorded before the first GitHub Actions build.

Verified locally:

- `go test ./...`, `go test -race ./...`, `go vet ./...`, `git diff --check`.
- Real temporary SQLite files: reopen, duplicate suppression, out-of-order OHLC,
  1m / 5m / 15m / 1h / 1d boundaries, sparse history, independent retention,
  unavailable metrics, and bounded raw time-bin queries.
- Procfs fixtures: CPU guest accounting, memory fallback, zero swap, source
  recovery, device resets/hotplug, interface selection, partition exclusion.
- `make build release`: native macOS demo binary and CGO-free Linux binaries.
  `file dist/syskline-linux-*` confirms statically linked amd64 and arm64 ELF.
  Stripped binaries are approximately 7.5–7.7 MiB; this is binary size, not RSS.
- Both Linux binaries successfully ran `--once --interval 250ms` as UID 65534 in
  temporary read-only, network-disabled Linux containers. amd64 used emulation
  on the ARM64 host; this is not an x86 hardware performance measurement.
- `python3 scripts/smoke_tui.py`: actual PTY metric/period/line/history/pause/help,
  shrink/restore and clean quit checks.
- The same PTY smoke ran the real ARM64 Linux collector via a temporary Docker
  wrapper with a temporary writable history directory. It recorded multiple
  samples while paused; the resulting SQLite database passed `integrity_check`.
- `scripts/install.sh` installed to a temporary prefix with mode 0755. The
  installed executable ran `--version` and `--demo --once` with the example TOML.
- Independent read-only review found issues in page navigation, partial failed
  samples and missing-value handling; these now have regression tests. A 600,000
  row review experiment confirmed point queries no longer scan NULL-filled bins.

Intentional MVP limits: foreground collection only; system-wide procfs metrics
(not cgroup quota metrics); aggregate CPU, selected aggregate disks/interfaces;
raw line history ends at the raw retention boundary. No background daemon,
alerts, process table, per-core panels, or remote agent. Host history should use
one database per host/collector. No broad performance or power-use claim is made
from these smoke checks.

## Candlestick detail and seconds update

- Added raw-sample OHLC at 1s / 5s / 15s, defaulting to 5s. Tests cover out-of-order
  millisecond samples, exact second boundaries, NULL gaps, and seconds expiring
  with raw retention while minute bars remain available.
- The 1s view samples at 250ms. Changing cadence replaces the logical timer and
  starts a sample immediately; obsolete timers cannot create parallel collectors.
- Default spacing reserves five columns per candle. Hollow rising bodies, solid
  falling bodies, half-cell edges, doji crosses and thin wicks are distinct in
  Unicode and ASCII. Rounded ticks preserve detail below 1.0 as well.
- Added close/cursor value tags, MA5, quote change, OHLC change, range, counts and
  countdown. MA5 loads four preceding bars even at 4-bar zoom, and invisible
  lookback candles do not stretch the chart's price scale.
- `go test ./...`, `go test -race ./...`, `go vet ./...`, native build and both
  static Linux builds passed after the update. Updated PTY smoke passed for demo
  and real Linux collection, including the seconds key and MA toggle. The Linux
  run recorded 7 samples and passed SQLite integrity checking.
- Inspected live 120x36 terminal output and deterministic 80x24 / 120x36 rendered
  frames. Text previews are saved in `docs/previews/`; README includes the smaller
  frame. No GUI or browser is part of the product or these checks.

## Comprehensive terminal overview

- Default `--view overview`, with six panels exposing all ten metrics. The layout
  switches between a 2x3 grid (>=100x36), a watchlist plus focused chart (>=80x22),
  and a compact table down to 48x16. Enter/o/Esc preserve access to chart detail.
- AllCandles/AllPoints batch readers share one raw scan, one minute rollup or one
  indexed seek per point bin. Tests compare all metrics with single-metric readers
  and check literal OHLC, subsecond ordering, NULL bins and retention boundaries.
- Overview tests cover focus, shared periods, stale replies, pause, unavailable
  values, all layout sizes and ASCII. Historical cursor navigation keeps disk
  write, network TX and all three load values visible; a regression test covers it.
- Inspected actual rendered frames at 120x40, 80x24 and 60x20, stored under
  `docs/previews/overview-*.txt`. Rate quote tags now preserve the complete unit.
- Full tests, race detector, vet, native build, both static Linux builds and demo
  PTY checks passed. The real Linux ARM64 PTY check passed with 7 persisted samples,
  20 minute metric bars and a successful SQLite integrity check.
