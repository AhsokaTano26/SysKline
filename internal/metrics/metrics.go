// Package metrics defines the shared metrics and Linux sampling model.
package metrics

import (
	"fmt"
	"time"
)

const (
	CPU = iota
	Memory
	Swap
	DiskRead
	DiskWrite
	NetRX
	NetTX
	Load1
	Load5
	Load15
	Count
)

type Metric struct{ Key, Label, Unit string }

var All = [Count]Metric{
	{"cpu", "CPU", "%"}, {"memory", "MEM", "%"}, {"swap", "SWAP", "%"},
	{"disk_read", "DISK R", "B/s"}, {"disk_write", "DISK W", "B/s"},
	{"net_rx", "NET RX", "B/s"}, {"net_tx", "NET TX", "B/s"},
	{"load1", "LOAD 1", ""}, {"load5", "LOAD 5", ""}, {"load15", "LOAD 15", ""},
}

var Periods = []time.Duration{time.Second, 5 * time.Second, 15 * time.Second, time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 24 * time.Hour}
var PeriodNames = []string{"1s", "5s", "15s", "1m", "5m", "15m", "1h", "1d"}

func Index(key string) int {
	for i, m := range All {
		if m.Key == key {
			return i
		}
	}
	return -1
}

func PeriodIndex(name string) int {
	for i, p := range PeriodNames {
		if p == name {
			return i
		}
	}
	return -1
}

type Sample struct {
	Time        time.Time      `json:"time"`
	Values      [Count]float64 `json:"values"`
	MemoryTotal uint64         `json:"memory_total_bytes"`
	SwapTotal   uint64         `json:"swap_total_bytes"`
	Uptime      float64        `json:"uptime_seconds"`
	Warnings    []string       `json:"warnings,omitempty"`
	Unavailable [Count]bool    `json:"-"`
}

type Source interface{ Sample() (Sample, error) }

// Candle timestamps are UTC bucket starts; Count includes only observed samples.
type Candle struct {
	Time                   time.Time
	Open, High, Low, Close float64
	Count                  int64
}

type Point struct {
	Time  time.Time
	Value float64
}

func Format(metric int, v float64) string {
	if metric >= DiskRead && metric <= NetTX {
		return Bytes(v) + "/s"
	}
	if metric <= Swap {
		return fmt.Sprintf("%.1f%%", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func Bytes(v float64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f%s", v, units[i])
	}
	return fmt.Sprintf("%.1f%s", v, units[i])
}
