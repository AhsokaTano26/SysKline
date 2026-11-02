# Seconds and detailed candlesticks implementation plan

User correction: the current view does not look like candlesticks; add detail
and seconds. Continue the existing terminal-only MVP in the current working tree.

Design choices: default 5s candles, new 1s/5s/15s choices followed by existing
minute/day choices. Keep the 1s base sampling interval, adapt to 250ms for 1s bars.
Second bars aggregate real retained raw samples, with true O/H/L/C and NULL gaps;
no fabricated OHLC or conversion from minute bars. Existing SQLite schema remains
compatible and long-period history stays unchanged.

Default candle spacing targets 5 columns: three-column body, thin center wick,
and whitespace. Use half-cell Unicode edges for finer body resolution, hollow
red rising candles and solid green falling candles; doji crosses stay honest.
ASCII uses outlines/hash bodies and ordinary axes. Add current close line/value
tag, actual selected close crosshair, consecutive-bar MA5 with a toggle, seconds
axis labels, and quote/percent/range/countdown details. Keep no-data, small-screen,
history, pause and keyboard behavior. Update demo to contain real subsecond
synthetic samples; demo is always visibly synthetic and memory-only.

- [x] Tests first: second rollups and boundaries, fractional demo values, body /
      wick shape / doji / time labels / overlays, sampling cadence, defaults,
      period keys, terminal bounds and existing minute history.
- [x] Implement shared periods, strict config, raw second aggregation and dense
      recent demo samples; verify store/metrics/config/CLI suites.
- [x] Implement candle geometry, current-value/cursor/MA overlays and readable
      spacing, then update TUI quote/details/period controls and cadence.
- [x] Update help, README, example config, PTY script; run tests/race/vet, inspect
      actual 80x24 and 120x36 rendered terminal frames, check both Linux builds.

Review focus: 1s bars need multiple samples; seconds history cannot exceed raw
retention; no same-value fake wicks; default spacing must survive 80 columns;
numeric old shortcuts/tests must track the new period ordering; percent change
must handle an open value of zero; errors/resize/ASCII must remain bounded.

Completed with regression coverage for review findings (MA lookback and timer
replacement). Verification and limitations are recorded in `docs/verification.md`.
