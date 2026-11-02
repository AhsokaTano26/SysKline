package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/chart"
	"github.com/AhsokaTano26/SysKline/internal/metrics"
	"github.com/charmbracelet/x/ansi"
)

const watchWidth = 28
const (
	overviewTable = iota
	overviewFocus
	overviewGrid
)

type metricGroup struct {
	name    string
	members []int
}

var overviewGroups = []metricGroup{
	{"CPU", []int{metrics.CPU}}, {"MEM", []int{metrics.Memory}},
	{"DISK I/O", []int{metrics.DiskRead, metrics.DiskWrite}}, {"NETWORK", []int{metrics.NetRX, metrics.NetTX}},
	{"SWAP", []int{metrics.Swap}}, {"LOAD", []int{metrics.Load1, metrics.Load5, metrics.Load15}},
}

func (m *Model) overviewLayout() int {
	if m.width >= 100 && m.height >= 36 {
		return overviewGrid
	}
	if m.width >= 80 && m.height >= 22 {
		return overviewFocus
	}
	return overviewTable
}
func (m *Model) chartViewportWidth() int {
	if !m.overview {
		return m.width
	}
	switch m.overviewLayout() {
	case overviewGrid:
		return (m.width-1)/2 - 2
	case overviewFocus:
		return m.width - watchWidth - 3
	default:
		return m.width
	}
}
func (m *Model) groupForMetric(metric int) int {
	for i, g := range overviewGroups {
		for _, member := range g.members {
			if member == metric {
				return i
			}
		}
	}
	return 0
}
func (m *Model) groupMetric(group int) int {
	for _, metric := range overviewGroups[group].members {
		if metric == m.metric {
			return metric
		}
	}
	return overviewGroups[group].members[0]
}
func (m *Model) dashboardCandle(metric int) (metrics.Candle, bool) {
	bars := m.dashboard[metric].bars
	for i := len(bars) - 1; i >= 0; i-- {
		b := bars[i]
		if b.Time.Before(m.windowStart()) || !b.Time.Before(m.windowEnd()) {
			continue
		}
		if !m.cursor.IsZero() && !b.Time.Equal(m.cursor) {
			continue
		}
		return b, true
	}
	return metrics.Candle{}, false
}
func (m *Model) dashboardNumber(metric int) (float64, bool) {
	if m.live {
		if m.latest.Time.IsZero() || m.latest.Unavailable[metric] {
			return 0, false
		}
		return m.latest.Values[metric], true
	}
	if b, ok := m.dashboardCandle(metric); ok {
		return b.Close, true
	}
	return 0, false
}
func (m *Model) dashboardValue(metric int) string {
	if value, ok := m.dashboardNumber(metric); ok {
		return metrics.Format(metric, value)
	}
	return "n/a"
}
func (m *Model) dashboardColor(metric int) string {
	if _, ok := m.dashboardNumber(metric); !ok {
		return m.cfg.Colors.Muted
	}
	if b, ok := m.dashboardCandle(metric); ok {
		if b.Close > b.Open {
			return m.cfg.Colors.Up
		}
		if b.Close < b.Open {
			return m.cfg.Colors.Down
		}
	}
	return m.cfg.Colors.Text
}

func (m *Model) overviewView() string {
	badge := m.status()
	if m.demo {
		badge = "DEMO / " + badge
	}
	top := joinSides(m.paint(" SYSKLINE / OVERVIEW", m.cfg.Colors.Accent, true)+" | "+m.host, m.paint(badge, m.cfg.Colors.Accent, true)+" "+m.clock().Local().Format("15:04:05"), m.width)
	uptime := time.Duration(m.latest.Uptime) * time.Second
	host := fmt.Sprintf(" UP %dd%02dh | RAM %s | SWAP %s | 10 METRICS | tick %s", int(uptime.Hours())/24, int(uptime.Hours())%24, metrics.Bytes(float64(m.latest.MemoryTotal)), metrics.Bytes(float64(m.latest.SwapTotal)), m.sampleInterval())
	kind := "CANDLES"
	if m.line {
		kind = "LINE"
	}
	right := kind + " | focus " + metrics.All[m.metric].Label
	toolbar := joinSides(m.periodStrip(m.width-ansi.StringWidth(right)-1), right, m.width)
	lines := []string{top, m.paint(host, m.cfg.Colors.Muted, false), toolbar}
	bodyHeight := m.height - 5
	switch m.overviewLayout() {
	case overviewGrid:
		leftWidth := (m.width - 1) / 2
		rightWidth := m.width - leftWidth - 1
		remaining := bodyHeight - 2
		for row := 0; row < 3; row++ {
			height := remaining / 3
			if row < remaining%3 {
				height++
			}
			left := m.overviewCard(row*2, leftWidth, height)
			right := m.overviewCard(row*2+1, rightWidth, height)
			for i := 0; i < height; i++ {
				lines = append(lines, left[i]+" "+right[i])
			}
			if row < 2 {
				lines = append(lines, "")
			}
		}
	case overviewFocus:
		left := m.box(m.watchRows(watchWidth-2), watchWidth, bodyHeight, false)
		right := m.overviewCard(m.groupForMetric(m.metric), m.width-watchWidth-1, bodyHeight)
		for i := 0; i < bodyHeight; i++ {
			lines = append(lines, left[i]+" "+right[i])
		}
	default:
		rows := m.watchRows(m.width)
		for len(rows) < bodyHeight {
			rows = append(rows, "")
		}
		lines = append(lines, rows[:bodyHeight]...)
	}
	notice := fmt.Sprintf(" Window %s - %s | %s bars | %s", m.windowStart().Local().Format("15:04:05"), m.windowEnd().Local().Format("15:04:05"), metrics.PeriodNames[m.period], m.status())
	if m.demo {
		notice += " | SYNTHETIC DEMO DATA"
	}
	if !m.cursor.IsZero() {
		notice += " | at " + m.cursor.Local().Format("15:04:05")
	}
	if err := m.errorText(); err != "" {
		notice = m.paint(" "+err, m.cfg.Colors.Up, false)
	}
	lines = append(lines, notice)
	left := " Tab focus  Enter detail  o chart  [ ] period  Space pause"
	if m.width < 90 {
		left = " Tab focus  Enter detail  o chart"
	}
	lines = append(lines, m.paint(joinSides(left, "? help  q quit", m.width), m.cfg.Colors.Muted, false))
	return strings.Join(lines, "\n")
}

func (m *Model) overviewCard(group, width, height int) []string {
	metric := m.groupMetric(group)
	active := m.groupForMetric(m.metric) == group
	name := overviewGroups[group].name
	if active {
		name = "[" + name + "]"
	}
	if len(overviewGroups[group].members) > 1 {
		name += " / " + metrics.All[metric].Label
	}
	color := m.cfg.Colors.Muted
	if active {
		color = m.cfg.Colors.Accent
	}
	header := joinSides(m.paint(name, color, true), m.paint(m.dashboardValue(metric), m.dashboardColor(metric), true), width-2)
	info := m.overviewInfo(group, metric)
	o := chart.Options{Width: width - 2, Height: height - 4, Start: m.windowStart(), End: m.windowEnd(), Cursor: m.cursor, Period: m.duration(), Gap: 3 * m.sampleInterval(), Metric: metric, ShowLast: true, ShowMA: m.showMA, Line: m.line, ASCII: m.cfg.ASCII, NoColor: m.cfg.NoColor, Up: m.cfg.Colors.Up, Down: m.cfg.Colors.Down, Accent: m.cfg.Colors.Accent, Grid: m.cfg.Colors.Grid, Muted: m.cfg.Colors.Muted}
	history := m.dashboard[metric]
	lines := []string{header, m.paint(info, m.cfg.Colors.Muted, false)}
	lines = append(lines, strings.Split(chart.Render(history.bars, history.points, o), "\n")...)
	return m.box(lines, width, height, active)
}
func (m *Model) overviewInfo(group, metric int) string {
	if !m.cursor.IsZero() && len(overviewGroups[group].members) == 1 {
		if b, ok := m.dashboardCandle(metric); ok {
			return fmt.Sprintf("O %s H %s L %s C %s", metrics.Format(metric, b.Open), metrics.Format(metric, b.High), metrics.Format(metric, b.Low), metrics.Format(metric, b.Close))
		}
		return "No sample at " + m.cursor.Local().Format("15:04:05")
	}
	switch group {
	case 0:
		low, high := math.Inf(1), math.Inf(-1)
		for _, b := range m.dashboard[metric].bars {
			if b.Time.Before(m.windowStart()) || !b.Time.Before(m.windowEnd()) {
				continue
			}
			low = min(low, b.Low)
			high = max(high, b.High)
		}
		if !math.IsInf(low, 1) {
			return "L " + metrics.Format(metric, low) + "  H " + metrics.Format(metric, high) + "  | " + metrics.PeriodNames[m.period]
		}
		return "Waiting for CPU samples"
	case 1:
		return "Total " + metrics.Bytes(float64(m.latest.MemoryTotal)) + " | Swap " + m.dashboardValue(metrics.Swap)
	case 2:
		return "R " + m.dashboardValue(metrics.DiskRead) + "  W " + m.dashboardValue(metrics.DiskWrite)
	case 3:
		return "RX " + m.dashboardValue(metrics.NetRX) + "  TX " + m.dashboardValue(metrics.NetTX)
	case 4:
		return "Total " + metrics.Bytes(float64(m.latest.SwapTotal)) + " | Memory " + m.dashboardValue(metrics.Memory)
	case 5:
		return "1m " + m.dashboardValue(metrics.Load1) + "  5m " + m.dashboardValue(metrics.Load5) + "  15m " + m.dashboardValue(metrics.Load15)
	}
	return ""
}

// box draws exact terminal cell bounds; selected cards use the accent border.
func (m *Model) box(content []string, width, height int, active bool) []string {
	color := m.cfg.Colors.Grid
	if active {
		color = m.cfg.Colors.Accent
	}
	tl, tr, bl, br, horizontal, vertical := "╭", "╮", "╰", "╯", "─", "│"
	if m.cfg.ASCII {
		tl, tr, bl, br, horizontal, vertical = "+", "+", "+", "+", "-", "|"
	}
	lines := make([]string, 0, height)
	lines = append(lines, m.paint(tl+strings.Repeat(horizontal, width-2)+tr, color, false))
	for i := 0; i < height-2; i++ {
		text := ""
		if i < len(content) {
			text = ansi.Truncate(content[i], width-2, "")
		}
		text += strings.Repeat(" ", max(0, width-2-ansi.StringWidth(text)))
		lines = append(lines, m.paint(vertical, color, false)+text+m.paint(vertical, color, false))
	}
	lines = append(lines, m.paint(bl+strings.Repeat(horizontal, width-2)+br, color, false))
	return lines
}
func (m *Model) watchRows(width int) []string {
	lines := []string{m.paint(fmt.Sprintf("%-9s %9s %s", "METRIC", "LAST", "TREND"), m.cfg.Colors.Muted, false)}
	trendWidth := max(0, min(24, width-20))
	for metric, item := range metrics.All {
		label := item.Label
		if metric == m.metric {
			label = "[" + label + "]"
		}
		row := fmt.Sprintf("%-9s %9s ", label, m.dashboardValue(metric))
		color := m.dashboardColor(metric)
		if metric == m.metric {
			color = m.cfg.Colors.Accent
		}
		lines = append(lines, m.paint(row+m.overviewSparkline(metric, trendWidth), color, metric == m.metric))
	}
	if m.overviewLayout() == overviewFocus {
		lines = append(lines, "", m.paint("Enter: open detail", m.cfg.Colors.Muted, false), m.paint("[ ]: change period", m.cfg.Colors.Muted, false), m.paint("Space: freeze charts", m.cfg.Colors.Muted, false))
	}
	return lines
}
func (m *Model) overviewSparkline(metric, width int) string {
	if width <= 0 {
		return ""
	}
	glyphs := []rune("▁▂▃▄▅▆▇█")
	if m.cfg.ASCII {
		glyphs = []rune(".:-=+*#@")
	}
	output := []rune(strings.Repeat(" ", width))
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, b := range m.dashboard[metric].bars {
		if b.Time.Before(m.windowStart()) || !b.Time.Before(m.windowEnd()) {
			continue
		}
		lo = min(lo, b.Close)
		hi = max(hi, b.Close)
	}
	if math.IsInf(lo, 1) {
		return string(output)
	}
	span := m.windowEnd().Sub(m.windowStart())
	for _, b := range m.dashboard[metric].bars {
		if b.Time.Before(m.windowStart()) || !b.Time.Before(m.windowEnd()) {
			continue
		}
		x := min(width-1, int(float64(b.Time.Sub(m.windowStart()))/float64(span)*float64(width)))
		level := 3
		if hi > lo {
			level = int(math.Round((b.Close - lo) / (hi - lo) * 7))
		}
		output[x] = glyphs[max(0, min(7, level))]
	}
	return string(output)
}
