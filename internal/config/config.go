// Package config loads a strict, optional XDG TOML configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/metrics"
	"github.com/pelletier/go-toml/v2"
)

type Colors struct {
	Up     string `toml:"up"`
	Down   string `toml:"down"`
	Accent string `toml:"accent"`
	Text   string `toml:"text"`
	Muted  string `toml:"muted"`
	Grid   string `toml:"grid"`
}

type Config struct {
	Interval    string   `toml:"interval"`
	Database    string   `toml:"database"`
	RawDays     int      `toml:"raw_days"`
	HistoryDays int      `toml:"history_days"`
	Metric      string   `toml:"metric"`
	Period      string   `toml:"period"`
	ASCII       bool     `toml:"ascii"`
	NoColor     bool     `toml:"no_color"`
	Interfaces  []string `toml:"interfaces"`
	Disks       []string `toml:"disks"`
	Colors      Colors   `toml:"colors"`
}

func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "syskline", "config.toml")
}

func Default() Config {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share")
	}
	return Config{Interval: "1s", Database: filepath.Join(dir, "syskline", "history.db"), RawDays: 7, HistoryDays: 90, Metric: "cpu", Period: "5s", Colors: Colors{Up: "#f87171", Down: "#4ade80", Accent: "#60a5fa", Text: "#e2e8f0", Muted: "#94a3b8", Grid: "#334155"}}
}

func Load(path string) (Config, error) {
	c := Default()
	if path == "" {
		path = DefaultPath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return c, nil
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer f.Close()
	if err := toml.NewDecoder(f).DisallowUnknownFields().Decode(&c); err != nil {
		return c, fmt.Errorf("config %s: %w", path, err)
	}
	return c, c.Validate()
}

func (c Config) IntervalDuration() time.Duration { d, _ := time.ParseDuration(c.Interval); return d }

func (c Config) Validate() error {
	d, err := time.ParseDuration(c.Interval)
	if err != nil || d < 250*time.Millisecond || d > time.Minute {
		return fmt.Errorf("interval must be between 250ms and 1m")
	}
	if c.RawDays < 1 || c.RawDays > 3650 || c.HistoryDays < c.RawDays || c.HistoryDays > 3650 {
		return fmt.Errorf("retention requires 1 <= raw_days <= history_days <= 3650")
	}
	if metrics.Index(c.Metric) < 0 {
		return fmt.Errorf("unknown metric %q", c.Metric)
	}
	if metrics.PeriodIndex(c.Period) < 0 {
		return fmt.Errorf("period must be %s", strings.Join(metrics.PeriodNames, ", "))
	}
	if strings.TrimSpace(c.Database) == "" {
		return fmt.Errorf("database path cannot be empty")
	}
	color := regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	for _, v := range []string{c.Colors.Up, c.Colors.Down, c.Colors.Accent, c.Colors.Text, c.Colors.Muted, c.Colors.Grid} {
		if !color.MatchString(v) {
			return fmt.Errorf("colors must be #RRGGBB hex values")
		}
	}
	for _, name := range append(append([]string{}, c.Disks...), c.Interfaces...) {
		if name == "" || strings.ContainsAny(name, "/\x00\n\r\t ") {
			return fmt.Errorf("invalid device name %q", name)
		}
	}
	return nil
}
