// SysKline is a Linux terminal system monitor with financial OHLC charts.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/config"
	"github.com/AhsokaTano26/SysKline/internal/metrics"
	"github.com/AhsokaTano26/SysKline/internal/store"
	"github.com/AhsokaTano26/SysKline/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

var version = "dev"

type options struct {
	cfg                       config.Config
	demo, once, help, version bool
}

func parse(args []string, out io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("syskline", flag.ContinueOnError)
	fs.SetOutput(out)
	var path, db, interval, metric, period, view string
	var ascii, noColor bool
	fs.StringVar(&path, "config", "", "TOML config file (default: "+config.DefaultPath()+")")
	fs.StringVar(&db, "db", "", "SQLite history file (overrides config)")
	fs.StringVar(&interval, "interval", "", "base sample interval, 250ms..1m (default 1s; 1s candles use 250ms)")
	fs.StringVar(&metric, "metric", "", "initial metric: cpu, memory, swap, disk_read, disk_write, net_rx, net_tx, load1, load5, load15")
	fs.StringVar(&period, "period", "", "initial candle period: 1s, 5s (default), 15s, 1m, 5m, 15m, 1h, 1d")
	fs.StringVar(&view, "view", "", "initial view: overview (default) or chart")
	fs.BoolVar(&ascii, "ascii", false, "use ASCII chart characters")
	fs.BoolVar(&noColor, "no-color", false, "disable ANSI colors")
	fs.BoolVar(&o.demo, "demo", false, "synthetic demo with isolated in-memory history")
	fs.BoolVar(&o.once, "once", false, "print one JSON snapshot and exit (no database writes)")
	fs.BoolVar(&o.help, "help", false, "show help")
	fs.BoolVar(&o.help, "h", false, "show help")
	fs.BoolVar(&o.version, "version", false, "show version")
	fs.Usage = func() {
		fmt.Fprint(out, "SysKline - Linux performance, in candlesticks\n\nUsage: syskline [flags]\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if o.help {
		fs.Usage()
		return o, nil
	}
	if o.version {
		return o, nil
	}
	c, err := config.Load(path)
	if err != nil {
		return o, err
	}
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	if visited["db"] {
		c.Database = db
	}
	if visited["interval"] {
		c.Interval = interval
	}
	if visited["metric"] {
		c.Metric = metric
	}
	if visited["period"] {
		c.Period = period
	}
	if visited["view"] {
		c.View = view
	}
	if os.Getenv("TERM") == "dumb" {
		c.ASCII = true
		c.NoColor = true
	}
	if os.Getenv("NO_COLOR") != "" {
		c.NoColor = true
	}
	if visited["ascii"] {
		c.ASCII = ascii
	}
	if visited["no-color"] {
		c.NoColor = noColor
	}
	if o.demo && visited["db"] {
		return o, fmt.Errorf("--demo uses in-memory history; remove --db")
	}
	if o.demo {
		c.Database = ":memory:"
	}
	if err := c.Validate(); err != nil {
		return o, err
	}
	o.cfg = c
	return o, nil
}

func run(args []string, out, stderr io.Writer) error {
	o, err := parse(args, out)
	if err != nil {
		return err
	}
	if o.help {
		return nil
	}
	if o.version {
		_, err = fmt.Fprintf(out, "syskline %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return err
	}
	var source metrics.Source
	if o.demo {
		source = metrics.Demo{}
	} else {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("live collection requires Linux; use --demo on %s", runtime.GOOS)
		}
		source = metrics.NewCollector("/proc", "/sys", o.cfg.Interfaces, o.cfg.Disks)
		// Prime cumulative counters so the first displayed rate is measured.
		if _, err = source.Sample(); err != nil {
			return err
		}
		if o.once {
			time.Sleep(o.cfg.IntervalDuration())
		}
	}
	host, _ := os.Hostname()
	if o.once {
		s, err := source.Sample()
		if err != nil {
			return err
		}
		type value struct {
			Value *float64 `json:"value"`
			Unit  string   `json:"unit"`
		}
		values := map[string]value{}
		for i, m := range metrics.All {
			var v *float64
			if !s.Unavailable[i] {
				measured := s.Values[i]
				v = &measured
			}
			values[m.Key] = value{v, m.Unit}
		}
		mode := "live"
		if o.demo {
			mode = "demo"
		}
		snapshot := struct {
			Version     string           `json:"version"`
			Mode        string           `json:"mode"`
			Host        string           `json:"host"`
			Time        time.Time        `json:"time"`
			Metrics     map[string]value `json:"metrics"`
			MemoryTotal uint64           `json:"memory_total_bytes"`
			SwapTotal   uint64           `json:"swap_total_bytes"`
			Uptime      float64          `json:"uptime_seconds"`
			Warnings    []string         `json:"warnings,omitempty"`
		}{version, mode, host, s.Time, values, s.MemoryTotal, s.SwapTotal, s.Uptime, s.Warnings}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(snapshot)
	}
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("interactive mode needs a terminal; use --once for JSON output")
	}
	db, err := store.Open(o.cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()
	if err = db.Cleanup(time.Now(), o.cfg.RawDays, o.cfg.HistoryDays); err != nil {
		return fmt.Errorf("clean history: %w", err)
	}
	if o.demo {
		fmt.Fprintln(stderr, "Preparing synthetic demo history...")
		if err = seedDemo(db, time.Now()); err != nil {
			return err
		}
	}
	model := tui.New(o.cfg, db, source, host, o.demo)
	_, err = tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(out)).Run()
	return err
}

func seedDemo(db *store.Store, now time.Time) error {
	boundary := now.Add(-3 * time.Hour).Truncate(time.Minute)
	for t := now.Add(-8 * 24 * time.Hour).Truncate(5 * time.Minute); t.Before(boundary); t = t.Add(5 * time.Minute) {
		if err := db.Insert(metrics.DemoAt(t)); err != nil {
			return err
		}
	}
	dense := now.Add(-10 * time.Minute).Truncate(time.Second)
	for t := boundary; t.Before(dense); t = t.Add(10 * time.Second) {
		if err := db.Insert(metrics.DemoAt(t)); err != nil {
			return err
		}
	}
	for t := dense; !t.After(now); t = t.Add(250 * time.Millisecond) {
		if err := db.Insert(metrics.DemoAt(t)); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "syskline:", err)
		os.Exit(1)
	}
}
