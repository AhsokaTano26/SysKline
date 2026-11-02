# SysKline MVP design

User brief: a Linux terminal performance monitor with financial OHLC charts, Go,
Bubble Tea, Lip Gloss, SQLite, Linux amd64/arm64 single binaries, and no web service.

## Architecture and choices

Implement a single foreground process. Read Linux procfs and sysfs without root;
use explicit device filters and counter deltas for CPU, disk and network rates.
The first read establishes rate baselines. Missing optional sources are reported
and stored as NULL with no corresponding OHLC update, never as a measured zero.
Use pure Go modernc SQLite and Bubble Tea v1 / Lip Gloss v1, with pinned modules.
The native Linux collector is separate from the deterministic demo source, which
also works on macOS and never opens the production database by default.

Alternatives: gopsutil offers more OS support but adds unnecessary scope for this
Linux tool; a separate daemon offers continuous collection but complicates an MVP.
Direct procfs collection and a foreground process fit the supplied requirements.
These implementation choices are defaults selected by the implementer.

## Data and retention

Sample every second by default. Store one wide raw row per timestamp and ten
minute OHLC rows (CPU, memory, swap, disk read/write, network receive/transmit,
load 1/5/15). Update OHLC transactionally with first/last timestamp ordering.
Derive 1m, 5m, 15m, 1h, 1d candles from UTC-aligned minute rows. Never invent
missing samples or fill gaps with zero. Retain raw samples for 7 days and minute
bars for 90 days by default; clean on startup and hourly. SQLite WAL, bounded
cache, indexed time queries, and incremental vacuum bound work and reclaim pages.
Collection only occurs while SysKline runs; no background daemon in this MVP.

## Terminal experience

Dark trading-terminal layout: identity and host summary, metric tabs and period
selector, large chart with bodies/wicks, numeric/time axes, selected OHLC, latest
system metrics, and keyboard hints. Red means increasing, green decreasing.
No-data and partial current candle are explicit. Arrow keys select candles;
moving beyond a viewport scrolls it, page keys jump, Home returns live. +/- change
visible candle count; [] change period; Tab switches metric; l toggles a true
sampled line chart (last actual raw sample per time bin for the time window, bounded indexed queries); Space freezes
the viewport while collection continues; ? shows help. Terminal resize always
clips output to the available cell dimensions. ASCII and no-color supported.

## CLI and configuration

`syskline` enters the TUI. Flags include --config, --db, --interval, --ascii,
--no-color, --demo, --once (JSON snapshot), --version, --help. TOML configuration
uses XDG config/data directories with home fallbacks; explicit files must exist.
Validate intervals, retention, metric, period, colors and unknown config keys.
CLI overrides config. Do not create a config automatically. Demo always uses in-memory
SQLite and clearly labels synthetic data; --demo --db is rejected to protect real
history. No browser, HTTP listener, telemetry, remote agent or root access.

## Delivery and validation

README (Chinese), example config, Makefile, install/build scripts, CI, meaningful
tests for OHLC ordering/rollup/cleanup, procfs fixtures and resets, invalid config,
chart geometry and ASCII, keyboard history/zoom/pause, and CLI smoke checks.
Build both Linux targets with CGO_ENABLED=0. Exercise the native Linux collector
in Linux when the local runtime allows, and test interactive exit/resize via PTY.
