package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/chart"
	"github.com/AhsokaTano26/SysKline/internal/metrics"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) paint(text, color string, bold bool) string {
	if m.cfg.NoColor {
		return text
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(bold).Render(text)
}
func (m *Model) latestValue(metric int) string {
	if m.latest.Time.IsZero() || m.latest.Unavailable[metric] {
		return "n/a"
	}
	return metrics.Format(metric, m.latest.Values[metric])
}
func percentChange(delta, base float64) string {
	if base == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%+.2f%%", 100*delta/base)
}
func deltaText(metric int, delta float64) string {
	if metric <= metrics.Swap {
		return fmt.Sprintf("%+.2fpp", delta)
	}
	if metric >= metrics.DiskRead && metric <= metrics.NetTX {
		sign := "+"
		if delta < 0 {
			sign = "-"
		}
		return sign + metrics.Format(metric, math.Abs(delta))
	}
	return fmt.Sprintf("%+.2f", delta)
}
func (m *Model) quoteLine() string {
	name := metrics.All[m.metric].Label
	if len(m.bars) == 0 || m.bars[len(m.bars)-1].Time.Before(m.windowStart()) {
		return " " + name + "  waiting for actual OHLC samples..."
	}
	last := m.bars[len(m.bars)-1]
	base := last.Open
	if len(m.bars) > 1 {
		base = m.bars[len(m.bars)-2].Close
	}
	delta := last.Close - base
	color := m.cfg.Colors.Up
	if delta < 0 {
		color = m.cfg.Colors.Down
	}
	quote := fmt.Sprintf(" %s  %s  %s (%s)", name, metrics.Format(m.metric, last.Close), deltaText(m.metric, delta), percentChange(delta, base))
	if m.showMA && !m.line {
		if ma, ok := chart.MovingAverage(m.bars, len(m.bars)-1, 5, m.duration()); ok {
			quote += "  MA5 " + metrics.Format(m.metric, ma)
		}
	}
	label := "CLOSED"
	if m.paused {
		label = "FROZEN"
	} else if left := last.Time.Add(m.duration()).Sub(m.clock()); left > 0 {
		label = fmt.Sprintf("%.1fs left", left.Seconds())
	}
	return joinSides(m.paint(quote, color, true), m.paint(label, m.cfg.Colors.Accent, false), m.width)
}

func (m *Model) periodStrip(budget int) string {
	render := func(first, last int) string {
		var parts []string
		for i := first; i < last; i++ {
			p := metrics.PeriodNames[i]
			if i == m.period {
				p = m.paint("["+p+"]", m.cfg.Colors.Accent, true)
			}
			parts = append(parts, p)
		}
		return " " + strings.Join(parts, "  ")
	}
	all := render(0, len(metrics.PeriodNames))
	if ansi.StringWidth(all) <= budget {
		return all
	}
	return render(max(0, m.period-1), min(len(metrics.PeriodNames), m.period+2))
}

func (m *Model) View() string {
	if m.width < 48 || m.height < 16 {
		return m.fit(fmt.Sprintf("SysKline %s\nTerminal %dx%d; use at least 48x16\nq quit  ? help", m.status(), m.width, m.height))
	}
	if m.help {
		return m.fit(m.helpView())
	}
	color := m.cfg.Colors
	badge := m.status()
	if m.demo {
		badge = "DEMO / " + badge
	}
	identity := m.paint(" SYSKLINE ", color.Accent, true) + m.paint(" | "+m.host, color.Text, false)
	top := joinSides(identity, m.paint(badge, color.Accent, true)+"  "+m.clock().Local().Format("15:04:05"), m.width)
	uptime := time.Duration(m.latest.Uptime) * time.Second
	summary := fmt.Sprintf(" CPU %s  MEM %s/%s  SWAP %s  LOAD %s/%s/%s  UP %dd%02dh", m.latestValue(metrics.CPU), m.latestValue(metrics.Memory), metrics.Bytes(float64(m.latest.MemoryTotal)), m.latestValue(metrics.Swap), m.latestValue(metrics.Load1), m.latestValue(metrics.Load5), m.latestValue(metrics.Load15), int(uptime.Hours())/24, int(uptime.Hours())%24)
	tabs := make([]string, metrics.Count)
	for i, metric := range metrics.All {
		text := " " + metric.Label + " "
		if i == m.metric {
			text = m.paint("["+metric.Label+"]", color.Accent, true)
		} else {
			text = m.paint(text, color.Muted, false)
		}
		tabs[i] = text
	}
	tabLine := strings.Join(tabs, "")
	if ansi.StringWidth(tabLine) > m.width {
		tabLine = strings.Join(tabs[max(0, m.metric-1):], "")
	}
	mode := "OHLC"
	if m.line {
		mode = "LINE"
	}
	scale := fmt.Sprintf("%s %d bars  %s", mode, m.visible(), metrics.All[m.metric].Unit)
	toolbar := joinSides(m.periodStrip(m.width-ansi.StringWidth(scale)-1), scale, m.width)
	divider := strings.Repeat("─", m.width)
	if m.cfg.ASCII {
		divider = strings.Repeat("-", m.width)
	}
	divider = m.paint(divider, color.Grid, false)
	chartHeight := max(5, m.height-12)
	o := chart.Options{Width: m.width, Height: chartHeight, Start: m.windowStart(), End: m.windowEnd(), Cursor: m.cursor, Period: m.duration(), Gap: 3 * m.sampleInterval(), Line: m.line, ASCII: m.cfg.ASCII, NoColor: m.cfg.NoColor, ShowLast: true, ShowMA: m.showMA, Metric: m.metric, Up: color.Up, Down: color.Down, Accent: color.Accent, Grid: color.Grid, Muted: color.Muted}
	plot := chart.Render(m.bars, m.points, o)
	ohlc := " No candle at " + m.selectedTime().Local().Format("01/02 15:04:05") + " | waiting for samples"
	detail := " Seconds use raw history; blank time buckets have no recorded samples."
	if b, ok := m.selected(); ok {
		format := func(v float64) string { return metrics.Format(m.metric, v) }
		ohlc = fmt.Sprintf(" O %s   H %s   L %s   C %s", format(b.Open), format(b.High), format(b.Low), format(b.Close))
		delta := b.Close - b.Open
		clr := color.Up
		if delta < 0 {
			clr = color.Down
		}
		label := "CLOSED"
		if b.Time.Add(m.duration()).After(m.clock()) {
			label = "FORMING"
		}
		if m.paused {
			label = "FROZEN"
		}
		detail = fmt.Sprintf(" %s %s  n=%d  %s (%s)  R %s", b.Time.Local().Format("01/02 15:04:05"), label, b.Count, deltaText(m.metric, delta), percentChange(delta, b.Open), format(b.High-b.Low))
		detail = m.paint(detail, clr, false)
	}
	ioLine := fmt.Sprintf(" DISK R %s W %s  NET RX %s TX %s  tick %s", m.latestValue(metrics.DiskRead), m.latestValue(metrics.DiskWrite), m.latestValue(metrics.NetRX), m.latestValue(metrics.NetTX), m.sampleInterval())
	notice := " Hollow red = rise  Solid green = fall  |  m MA5  ? help  Home live  q quit"
	if m.demo {
		notice = " SYNTHETIC DEMO DATA  |  Hollow = rise  Solid = fall  |  ? help  q quit"
	}
	if err := m.errorText(); err != "" {
		notice = m.paint(" "+err, color.Up, false)
	}
	hints := " Tab metric  1-8 / [ ] period  <- -> cursor  +/- zoom  l line  Space pause"
	return m.fit(strings.Join([]string{top, m.paint(summary, color.Text, false), tabLine, toolbar, m.quoteLine(), divider, plot, divider, m.paint(ohlc, color.Text, true), detail, m.paint(ioLine, color.Muted, false), notice, m.paint(hints, color.Muted, false)}, "\n"))
}

func joinSides(left, right string, width int) string {
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return ansi.Truncate(left, max(0, width-ansi.StringWidth(right)-1), "") + " " + right
	}
	return left + strings.Repeat(" ", gap) + right
}
func (m *Model) fit(view string) string {
	lines := strings.Split(view, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i, line := range lines {
		if m.cfg.ASCII {
			var b strings.Builder
			for _, r := range line {
				if r > 127 {
					b.WriteByte('?')
				} else {
					b.WriteRune(r)
				}
			}
			line = b.String()
		}
		lines[i] = ansi.Truncate(line, m.width, "")
	}
	return strings.Join(lines, "\n")
}
func (m *Model) helpView() string {
	return strings.Join([]string{
		" SYSKLINE  /  TERMINAL MARKET FOR YOUR MACHINE", "",
		" Tab / Shift+Tab, Up/Down  Switch metric",
		" [ / ] or 1..8            1s 5s 15s 1m 5m 15m 1h 1d",
		" Left / Right             Select OHLC / pan at edge",
		" PgUp / PgDn              Scroll half a screen",
		" + / -                    Zoom in / out (bar count)",
		" Home / End / g           Return to live view",
		" Space                    Freeze view / resume live",
		" l                        Candles / sampled line",
		" m                        Toggle MA5 close average",
		" a                        Unicode / ASCII characters",
		" r                        Reload visible history",
		" ? / F1 / Esc             Toggle help / close help",
		" q / Ctrl+C               Quit", "",
		" Hollow red = rise. Solid green = fall. Cross = doji.",
		" Thin wick = actual high/low; C tag = last close.",
		" MA5 uses 5 consecutive closes; no gap filling.",
		" Seconds use retained raw samples; 1s ticks at 250ms.",
		" Quote change = previous close; OHLC change = open.",
		" UTC buckets, local time labels. No background daemon.",
		" History / pause / help still collect new samples.",
	}, "\n")
}
