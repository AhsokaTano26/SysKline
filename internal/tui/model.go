// Package tui implements the Bubble Tea state machine. Collection and SQL run in
// commands; rendering only reads the model. Query revisions discard stale views.
package tui

import (
	"fmt"
	"math"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/chart"
	"github.com/AhsokaTano26/SysKline/internal/config"
	"github.com/AhsokaTano26/SysKline/internal/metrics"
	"github.com/AhsokaTano26/SysKline/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

type sampleTick struct{ generation uint64 }
type sampleMsg struct {
	sample metrics.Sample
	err    error
	valid  bool // Source succeeded even if a subsequent database operation failed.
}
type historyMsg struct {
	revision uint64
	bars     []metrics.Candle
	points   []metrics.Point
	err      error
}

type Model struct {
	cfg                      config.Config
	db                       *store.Store
	source                   metrics.Source
	host                     string
	demo                     bool
	width, height            int
	metric, period, zoom     int
	live, paused, line, help bool
	showMA                   bool
	anchor, cursor           time.Time
	latest                   metrics.Sample
	bars                     []metrics.Candle
	points                   []metrics.Point
	revision                 uint64
	collectErr, historyErr   string
	lastCleanup              time.Time
	sampling                 bool
	timerGeneration          uint64
}

func New(cfg config.Config, db *store.Store, source metrics.Source, host string, demo bool) *Model {
	return &Model{cfg: cfg, db: db, source: source, host: host, demo: demo, width: 100, height: 30, metric: metrics.Index(cfg.Metric), period: metrics.PeriodIndex(cfg.Period), live: true, showMA: true}
}

func (m *Model) Init() tea.Cmd { return tea.Batch(m.sampleCmd(), m.queryCmd()) }

func (m *Model) duration() time.Duration { return metrics.Periods[m.period] }
func (m *Model) visible() int {
	if m.zoom == 0 {
		return max(4, min(4092, chart.PlotWidth(m.width)/5))
	}
	return max(4, min(4092, m.zoom, max(4, chart.PlotWidth(m.width)/2)))
}
func (m *Model) sampleInterval() time.Duration {
	return max(250*time.Millisecond, min(m.cfg.IntervalDuration(), m.duration()/4))
}
func (m *Model) clock() time.Time {
	if !m.latest.Time.IsZero() {
		return m.latest.Time
	}
	return time.Now()
}
func (m *Model) windowEnd() time.Time {
	if !m.live && !m.anchor.IsZero() {
		return m.anchor
	}
	return m.clock().Truncate(m.duration()).Add(m.duration())
}
func (m *Model) windowStart() time.Time {
	return m.windowEnd().Add(-time.Duration(m.visible()) * m.duration())
}
func (m *Model) selectedTime() time.Time {
	if !m.cursor.IsZero() {
		return m.cursor
	}
	return m.windowEnd().Add(-m.duration())
}
func (m *Model) selected() (metrics.Candle, bool) {
	t := m.selectedTime()
	for _, b := range m.bars {
		if b.Time.Equal(t) {
			return b, true
		}
	}
	return metrics.Candle{}, false
}

func (m *Model) sampleCmd() tea.Cmd {
	if m.source == nil || m.sampling {
		return nil
	}
	m.sampling = true
	source, db := m.source, m.db
	cleanup := m.lastCleanup.IsZero() || time.Since(m.lastCleanup) >= time.Hour
	if cleanup {
		m.lastCleanup = time.Now()
	}
	cfg := m.cfg
	return func() tea.Msg {
		s, err := source.Sample()
		valid := err == nil
		if err == nil && db != nil {
			err = db.Insert(s)
		}
		if err == nil && db != nil && cleanup {
			err = db.Cleanup(time.Now(), cfg.RawDays, cfg.HistoryDays)
		}
		return sampleMsg{sample: s, err: err, valid: valid}
	}
}

func (m *Model) queryCmd() tea.Cmd {
	m.revision++
	if m.db == nil {
		return nil
	}
	db, metric, period, end, n, revision := m.db, m.metric, m.duration(), m.windowEnd(), m.visible(), m.revision
	line, width := m.line, m.width
	return func() tea.Msg {
		// Four preceding bars allow MA5 to start at the left edge and at 4-bar zoom.
		bars, err := db.Candles(metric, period, end.Add(-time.Nanosecond), n+4)
		var points []metrics.Point
		if err == nil && line {
			points, err = db.Points(metric, end.Add(-time.Duration(n)*period), end, min(4096, max(1, width-11)))
		}
		return historyMsg{revision: revision, bars: bars, points: points, err: err}
	}
}

func (m *Model) freeze() {
	if m.live {
		m.anchor = m.windowEnd()
		m.live = false
	}
}
func (m *Model) resetHistory() { m.bars = nil; m.points = nil; m.cursor = time.Time{} }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
		m.keepCursorVisible()
		return m, m.queryCmd()
	case sampleTick:
		if msg.generation != m.timerGeneration {
			return m, nil
		}
		return m, m.sampleCmd()
	case sampleMsg:
		m.sampling = false
		if msg.err != nil {
			m.collectErr = msg.err.Error()
		} else {
			m.collectErr = ""
		}
		if (msg.err == nil || msg.valid) && !msg.sample.Time.IsZero() {
			m.latest = msg.sample
		}
		var query tea.Cmd
		if m.live && !m.paused {
			query = m.queryCmd()
		}
		generation := m.timerGeneration
		return m, tea.Batch(query, tea.Tick(m.sampleInterval(), func(time.Time) tea.Msg { return sampleTick{generation: generation} }))
	case historyMsg:
		if msg.revision != m.revision {
			return m, nil
		}
		if msg.err != nil {
			m.historyErr = msg.err.Error()
		} else {
			m.historyErr = ""
			m.bars, m.points = msg.bars, msg.points
		}
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" || key == "q" {
			return m, tea.Quit
		}
		if key == "?" || key == "f1" {
			m.help = !m.help
			return m, nil
		}
		if key == "esc" {
			m.help = false
			return m, nil
		}
		if m.help {
			return m, nil
		}
		query := false
		previousInterval := m.sampleInterval()
		switch key {
		case "tab", "down", "j":
			m.metric = (m.metric + 1) % metrics.Count
			m.resetHistory()
			query = true
		case "shift+tab", "up", "k":
			m.metric = (m.metric + metrics.Count - 1) % metrics.Count
			m.resetHistory()
			query = true
		case "[", "]":
			delta := 1
			if key == "[" {
				delta = -1
			}
			end := m.windowEnd()
			m.period = (m.period + delta + len(metrics.Periods)) % len(metrics.Periods)
			if !m.live {
				m.anchor = end.Add(-time.Nanosecond).Truncate(m.duration()).Add(m.duration())
			}
			m.resetHistory()
			query = true
		case "1", "2", "3", "4", "5", "6", "7", "8":
			end := m.windowEnd()
			m.period = int(key[0] - '1')
			if !m.live {
				m.anchor = end.Add(-time.Nanosecond).Truncate(m.duration()).Add(m.duration())
			}
			m.resetHistory()
			query = true
		case "left", "h", "right":
			m.freeze()
			t := m.selectedTime()
			delta := -m.duration()
			if key == "right" {
				delta = m.duration()
			}
			m.cursor = t.Add(delta)
			latest := m.clock().Truncate(m.duration())
			if m.cursor.After(latest) {
				m.cursor = latest
			}
			m.keepCursorVisible()
			query = true
		case "pgup", "pgdown":
			m.freeze()
			shift := time.Duration(max(1, m.visible()/2)) * m.duration()
			if key == "pgup" {
				shift = -shift
			}
			selected := m.selectedTime()
			m.anchor = m.anchor.Add(shift)
			m.cursor = selected.Add(shift)
			latest := m.clock().Truncate(m.duration()).Add(m.duration())
			if m.anchor.After(latest) {
				m.anchor = latest
				m.cursor = latest.Add(-m.duration())
			}
			query = true
		case "home", "end", "g":
			m.live = true
			m.paused = false
			m.cursor = time.Time{}
			query = true
		case " ":
			if m.paused {
				m.paused = false
				m.live = true
				m.cursor = time.Time{}
				query = true
			} else {
				m.freeze()
				m.paused = true
				m.revision++
			}
		case "+", "=", "-":
			if key == "-" {
				m.zoom = min(4096, int(math.Ceil(float64(m.visible())*1.4)))
			} else {
				m.zoom = max(4, int(float64(m.visible())/1.4))
			}
			m.keepCursorVisible()
			query = true
		case "l":
			m.line = !m.line
			query = true
		case "a":
			m.cfg.ASCII = !m.cfg.ASCII
		case "m":
			m.showMA = !m.showMA
		case "r":
			query = true
		}
		var sample tea.Cmd
		if m.sampleInterval() != previousInterval {
			m.timerGeneration++
			sample = m.sampleCmd()
		}
		if query {
			return m, tea.Batch(m.queryCmd(), sample)
		}
		return m, sample
	}
	return m, nil
}

func (m *Model) keepCursorVisible() {
	if m.live || m.cursor.IsZero() {
		return
	}
	if m.cursor.Before(m.windowStart()) {
		m.anchor = m.cursor.Add(time.Duration(m.visible()) * m.duration())
	}
	if !m.cursor.Before(m.windowEnd()) {
		m.anchor = m.cursor.Add(m.duration())
	}
}

func (m *Model) status() string {
	if m.paused {
		return "PAUSED"
	}
	if !m.live {
		return "HISTORY"
	}
	return "LIVE"
}

func (m *Model) errorText() string {
	if m.collectErr != "" {
		return "SAMPLE / DB: " + m.collectErr
	}
	if m.historyErr != "" {
		return "HISTORY: " + m.historyErr
	}
	if len(m.latest.Warnings) > 0 {
		return "SOURCE: " + m.latest.Warnings[0]
	}
	return ""
}

func (m *Model) description() string {
	kind := "CANDLES"
	if m.line {
		kind = "LINE / raw samples"
	}
	return fmt.Sprintf("%s  %s  %s  %d bars", metrics.All[m.metric].Label, metrics.PeriodNames[m.period], kind, m.visible())
}
