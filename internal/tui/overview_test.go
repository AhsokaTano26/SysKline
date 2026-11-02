package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/config"
	"github.com/AhsokaTano26/SysKline/internal/metrics"
	"github.com/AhsokaTano26/SysKline/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func overviewFixture(t *testing.T) *Model {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	now := time.Date(2026, 11, 2, 12, 0, 17, 750000000, time.Local)
	for at := now.Add(-3 * time.Minute); !at.After(now); at = at.Add(250 * time.Millisecond) {
		if err := db.Insert(metrics.DemoAt(at)); err != nil {
			t.Fatal(err)
		}
	}
	c := config.Default()
	c.NoColor = true
	m := New(c, db, nil, "linux-node", true)
	m.latest = metrics.DemoAt(now)
	return m
}

func TestOverviewDefaultAndDetailRoundTrip(t *testing.T) {
	m := overviewFixture(t)
	if !m.overview {
		t.Fatal("overview is not the default entry")
	}
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(cmd())
	view := m.View()
	for _, label := range []string{"OVERVIEW", "[CPU]", "MEM", "DISK", "NETWORK", "SWAP", "LOAD", "RX", "TX", "5m", "15m"} {
		if !strings.Contains(view, label) {
			t.Errorf("overview missing %q", label)
		}
	}
	key(m, "tab")
	if m.metric != metrics.Memory {
		t.Fatal("focus did not change")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.overview {
		t.Fatal("Enter did not open detail")
	}
	m.Update(cmd())
	if !strings.Contains(m.View(), " O ") {
		t.Fatal("detail OHLC missing")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.overview {
		t.Fatal("Esc did not return to overview")
	}
	m.Update(cmd())
	if m.metric != metrics.Memory {
		t.Fatal("focused metric lost")
	}
	key(m, "o")
	if m.overview {
		t.Fatal("o did not toggle detail")
	}
}

func TestOverviewAllMetricsSharePeriodWindowAndPause(t *testing.T) {
	m := overviewFixture(t)
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(cmd())
	for metric := 0; metric < metrics.Count; metric++ {
		bars := m.dashboard[metric].bars
		if len(bars) == 0 {
			t.Fatalf("metric %d history absent", metric)
		}
		if !bars[len(bars)-1].Time.Equal(m.clock().Truncate(m.duration())) {
			t.Fatalf("metric %d window differs", metric)
		}
	}
	key(m, " ")
	end := m.windowEnd()
	value := m.dashboardValue(metrics.CPU)
	old := m.dashboard[metrics.CPU].bars[len(m.dashboard[metrics.CPU].bars)-1]
	m.Update(sampleMsg{sample: metrics.DemoAt(m.clock().Add(time.Minute))})
	if m.windowEnd() != end || m.dashboard[metrics.CPU].bars[len(m.dashboard[metrics.CPU].bars)-1] != old {
		t.Fatal("paused overview moved")
	}
	if m.dashboardValue(metrics.CPU) != value {
		t.Fatal("paused overview numbers changed")
	}
	m.Update(historyMsg{revision: m.revision - 1, batch: true})
	if len(m.dashboard[0].bars) == 0 {
		t.Fatal("stale batch replaced overview")
	}
}

func TestOverviewResponsiveASCIIAndUnavailableValues(t *testing.T) {
	m := overviewFixture(t)
	m.cfg.ASCII = true
	for _, size := range [][2]int{{48, 16}, {60, 20}, {80, 24}, {100, 36}, {120, 40}, {180, 50}} {
		_, cmd := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.Update(cmd())
		view := m.View()
		if len(strings.Split(view, "\n")) > size[1] {
			t.Fatalf("height overflow %v", size)
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("width overflow %v: %s", size, line)
			}
		}
		for _, r := range view {
			if r > 127 {
				t.Fatalf("non-ASCII %q", r)
			}
		}
		for _, label := range []string{"CPU", "MEM", "SWAP", "DISK", "NET", "LOAD"} {
			if !strings.Contains(view, label) {
				t.Fatalf("%v lost %s", size, label)
			}
		}
	}
	m.latest.Unavailable[metrics.NetRX] = true
	if m.dashboardValue(metrics.NetRX) != "n/a" {
		t.Fatal("unavailable metric shown as zero")
	}
}

func TestOverviewHistoryRetainsAllCompanionMetrics(t *testing.T) {
	m := overviewFixture(t)
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(cmd())
	key(m, "left")
	view := m.View()
	for _, label := range []string{"W " + m.dashboardValue(metrics.DiskWrite), "TX " + m.dashboardValue(metrics.NetTX), "5m " + m.dashboardValue(metrics.Load5), "15m " + m.dashboardValue(metrics.Load15)} {
		if !strings.Contains(view, label) {
			t.Errorf("history overview hid %q", label)
		}
	}
}
