package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/config"
	"github.com/AhsokaTano26/SysKline/internal/metrics"
	"github.com/AhsokaTano26/SysKline/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func key(m *Model, s string) {
	k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	switch s {
	case "left":
		k = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		k = tea.KeyMsg{Type: tea.KeyRight}
	case "home":
		k = tea.KeyMsg{Type: tea.KeyHome}
	case "tab":
		k = tea.KeyMsg{Type: tea.KeyTab}
	}
	m.Update(k)
}

func TestNavigationZoomPauseAndStaleQueries(t *testing.T) {
	c := config.Default()
	c.Period = "1m"
	c.NoColor = true
	m := New(c, nil, nil, "test-host", false)
	m.latest = metrics.DemoAt(time.Date(2026, 1, 1, 12, 0, 10, 0, time.UTC))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	oldEnd := m.windowEnd()
	key(m, "left")
	if m.live || m.cursor.IsZero() {
		t.Fatal("cursor must freeze viewport")
	}
	for i := 0; i < 100; i++ {
		key(m, "left")
	}
	if !m.windowEnd().Before(oldEnd) || m.cursor.Before(m.windowStart()) {
		t.Fatal("cursor must pan into history")
	}
	key(m, "home")
	if !m.live {
		t.Fatal("home must return live")
	}
	before := m.visible()
	key(m, "+")
	if m.visible() >= before {
		t.Fatal("zoom did not zoom in")
	}
	key(m, "tab")
	if m.metric != 1 {
		t.Fatal("metric switch")
	}
	key(m, "]")
	if metrics.PeriodNames[m.period] != "5m" {
		t.Fatal("period switch")
	}
	key(m, "l")
	if !m.line {
		t.Fatal("line toggle")
	}
	key(m, " ")
	frozen := m.windowEnd()
	m.Update(sampleMsg{sample: metrics.DemoAt(m.latest.Time.Add(5 * time.Minute))})
	if !m.paused || m.windowEnd() != frozen {
		t.Fatal("paused view drifted")
	}
	m.Update(historyMsg{revision: m.revision - 1, bars: []metrics.Candle{{Open: 999}}})
	if len(m.bars) != 0 {
		t.Fatal("stale query applied")
	}
	key(m, "home")
	if m.paused || !m.live {
		t.Fatal("resume failed")
	}
}

func TestViewsFitTerminalAndASCII(t *testing.T) {
	c := config.Default()
	c.ASCII = true
	c.NoColor = true
	m := New(c, nil, nil, "主机-with-a-very-long-name", true)
	m.latest = metrics.DemoAt(time.Now())
	for _, size := range [][2]int{{1, 1}, {20, 4}, {60, 16}, {80, 24}, {140, 40}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, help := range []bool{false, true} {
			m.help = help
			view := m.View()
			if len(strings.Split(view, "\n")) > size[1] {
				t.Fatalf("height overflow: %dx%d", size[0], size[1])
			}
			for _, line := range strings.Split(view, "\n") {
				if ansi.StringWidth(line) > size[0] {
					t.Fatalf("width overflow: %q", line)
				}
			}
			for _, r := range view {
				if r > 127 {
					t.Fatalf("ASCII output contains %q", r)
				}
			}
		}
	}
}

func TestPagePanAndFailedSamplePreserveState(t *testing.T) {
	c := config.Default()
	c.Period = "1m"
	m := New(c, nil, nil, "host", false)
	m.width = 200
	m.zoom = 32
	m.latest = metrics.DemoAt(time.Date(2026, 1, 1, 12, 0, 10, 0, time.UTC))
	before := m.selectedTime()
	m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	want := before.Add(-16 * time.Minute)
	if !m.selectedTime().Equal(want) {
		t.Fatalf("page moved selection twice: got %s want %s", m.selectedTime(), want)
	}
	last := m.latest
	m.Update(sampleMsg{sample: metrics.Sample{Time: last.Time.Add(time.Minute)}, err: fmt.Errorf("proc unavailable")})
	if m.latest.Time != last.Time || m.latest.Values != last.Values {
		t.Fatal("failed partial sample replaced last valid metrics")
	}
}

func TestSecondKeysCadenceAndDefaultSpacing(t *testing.T) {
	m := New(config.Default(), nil, nil, "host", false)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.visible() > 12 {
		t.Fatalf("default candles too narrow: %d bars", m.visible())
	}
	for _, tc := range []struct {
		key              string
		period, interval time.Duration
	}{{"1", time.Second, 250 * time.Millisecond}, {"2", 5 * time.Second, time.Second}, {"3", 15 * time.Second, time.Second}, {"4", time.Minute, time.Second}, {"8", 24 * time.Hour, time.Second}} {
		key(m, tc.key)
		if m.duration() != tc.period || m.sampleInterval() != tc.interval {
			t.Fatalf("%s: period %s cadence %s", tc.key, m.duration(), m.sampleInterval())
		}
	}
	key(m, "m")
	if m.showMA {
		t.Fatal("MA overlay toggle did not turn off")
	}
	key(m, "m")
	if !m.showMA {
		t.Fatal("MA overlay toggle did not turn on")
	}
}

func TestQuoteDetailsWithZeroOpenStayFinite(t *testing.T) {
	c := config.Default()
	c.NoColor = true
	m := New(c, nil, nil, "host", false)
	m.latest = metrics.DemoAt(time.Date(2026, 1, 1, 12, 0, 2, 0, time.UTC))
	m.bars = []metrics.Candle{{Time: m.latest.Time.Truncate(5 * time.Second), Open: 0, High: 10, Low: 0, Close: 10, Count: 8}}
	v := m.View()
	if strings.Contains(v, "NaN") || strings.Contains(v, "Inf") {
		t.Fatal("invalid percent change", v)
	}
	if !strings.Contains(v, "n/a") || !strings.Contains(v, "n=8") || !strings.Contains(v, "left") {
		t.Fatal("OHLC details or countdown missing", v)
	}
}

func TestMA5HasHistoryWhenZoomedToFourBars(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 8; i++ {
		s := metrics.Sample{Time: base.Add(time.Duration(i) * 5 * time.Second)}
		s.Values[0] = float64((i + 1) * 10)
		if err := db.Insert(s); err != nil {
			t.Fatal(err)
		}
	}
	c := config.Default()
	c.NoColor = true
	m := New(c, db, nil, "host", false)
	m.zoom = 4
	m.latest = metrics.DemoAt(base.Add(37 * time.Second))
	m.Update(m.queryCmd()())
	if !strings.Contains(m.quoteLine(), "MA5 60.0%") {
		t.Fatal("MA5 vanished at 4-bar zoom", m.quoteLine())
	}
}

func TestShorterPeriodStartsSamplingWithoutWaitingOldTimer(t *testing.T) {
	c := config.Default()
	c.Interval = "1m"
	c.Period = "1d"
	m := New(c, nil, metrics.Demo{}, "host", false)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if cmd == nil || !m.sampling {
		t.Fatal("switch to 1s did not start immediate sampling")
	}
	// The previous long-period timer must not start a competing collector loop.
	_, cmd = m.Update(sampleTick{generation: m.timerGeneration - 1})
	if cmd != nil {
		t.Fatal("obsolete timer started another collection")
	}
}
