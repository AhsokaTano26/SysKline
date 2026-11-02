// Package chart renders time-aligned OHLC candles and sampled lines in cells.
package chart

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/metrics"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const axisWidth = 9
const valueWidth = 13

type Options struct {
	Width, Height                 int
	Start, End, Cursor            time.Time
	Period, Gap                   time.Duration
	Line, ASCII, NoColor          bool
	ShowLast, ShowMA              bool
	Metric                        int
	Up, Down, Accent, Grid, Muted string
}

type cell struct {
	r    rune
	kind int
}

// PlotWidth excludes the numeric axis and the right-side quote/crosshair tags.
func PlotWidth(width int) int { return max(1, width-axisWidth-valueWidth) }

// MovingAverage uses consecutive complete time buckets, never filling a gap.
func MovingAverage(bars []metrics.Candle, index, length int, period time.Duration) (float64, bool) {
	if length < 1 || index >= len(bars) || index < length-1 {
		return 0, false
	}
	sum := 0.0
	for i := index - length + 1; i <= index; i++ {
		if i > index-length+1 && bars[i].Time.Sub(bars[i-1].Time) != period {
			return 0, false
		}
		sum += bars[i].Close
	}
	return sum / float64(length), true
}

func Render(bars []metrics.Candle, points []metrics.Point, o Options) string {
	if o.Width <= 0 || o.Height <= 0 {
		return ""
	}
	if o.Width < 24 || o.Height < 5 {
		return fit("Resize terminal", o.Width, o.Height)
	}
	w, h := PlotWidth(o.Width), o.Height-2
	span := o.End.Sub(o.Start)
	if span <= 0 {
		span = time.Minute
	}
	canvas := make([][]cell, h)
	for y := range canvas {
		canvas[y] = make([]cell, w)
		for x := range canvas[y] {
			canvas[y][x] = cell{' ', 0}
		}
	}
	put := func(x, y int, r rune, kind int) {
		if x >= 0 && x < w && y >= 0 && y < h {
			canvas[y][x] = cell{r, kind}
		}
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	include := func(v float64) {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			lo = math.Min(lo, v)
			hi = math.Max(hi, v)
		}
	}
	if o.Line {
		for _, p := range points {
			include(p.Value)
		}
	} else {
		for i, b := range bars {
			if b.Time.Before(o.Start) || !b.Time.Before(o.End) {
				continue
			}
			include(b.Low)
			include(b.High)
			if o.ShowMA {
				if value, ok := MovingAverage(bars, i, 5, o.Period); ok {
					include(value)
				}
			}
		}
	}
	empty := math.IsInf(lo, 1)
	percentMetric := o.Metric >= metrics.CPU && o.Metric <= metrics.Swap && hi <= 100
	if empty {
		lo, hi = 0, 100
	} else {
		pad := math.Max((hi-lo)*.10, math.Abs(hi)*.015)
		if pad == 0 {
			pad = .1
		}
		lo = math.Max(0, lo-pad)
		hi += pad
		if percentMetric {
			hi = math.Min(100, hi)
		}
	}
	// Use round value ticks rather than evenly spaced decimal row labels.
	rawStep := (hi - lo) / float64(max(2, min(6, h/3+1))-1)
	mag := math.Pow(10, math.Floor(math.Log10(rawStep)))
	step := mag
	best := math.Inf(1)
	for _, factor := range []float64{1, 2, 2.5, 5, 10} {
		if distance := math.Abs(mag*factor - rawStep); distance < best {
			step = mag * factor
			best = distance
		}
	}
	lo = math.Floor(lo/step) * step
	hi = math.Ceil(hi/step) * step
	if percentMetric {
		hi = math.Min(100, hi)
	}
	pixelY := func(v float64) int { return max(0, min(2*h-1, int(math.Round((hi-v)/(hi-lo)*float64(2*h-1))))) }
	ypos := func(v float64) int { return pixelY(v) / 2 }
	xpos := func(t time.Time) int { return max(0, min(w-1, int(float64(t.Sub(o.Start))/float64(span)*float64(w)))) }
	dash, vertical, horizontal, dot := '┄', '│', '─', '•'
	if o.ASCII {
		dash, vertical, horizontal, dot = '.', '|', '-', '*'
	}
	ticks := make(map[int]float64)
	for value := lo; value <= hi+step*.001; value += step {
		y := ypos(value)
		ticks[y] = value
		for x := 0; x < w; x++ {
			put(x, y, dash, 0)
		}
	}
	for i := 1; i < 4; i++ {
		x := w * i / 4
		for y := 0; y < h; y++ {
			if canvas[y][x].r == ' ' {
				r := '┊'
				if o.ASCII {
					r = ':'
				}
				put(x, y, r, 0)
			}
		}
	}
	segment := func(x1, y1, x2, y2 int, r rune, kind int) {
		steps := max(abs(x2-x1), abs(y2-y1))
		for i := 0; i <= steps; i++ {
			f := float64(i) / float64(max(1, steps))
			put(x1+int(math.Round(float64(x2-x1)*f)), y1+int(math.Round(float64(y2-y1)*f)), r, kind)
		}
	}
	// Indicators are drawn behind candles, so body and wick geometry stays visible.
	if o.ShowMA && !o.Line {
		var px, py int
		previous := false
		for i, b := range bars {
			value, ok := MovingAverage(bars, i, 5, o.Period)
			if !ok || b.Time.Before(o.Start) || !b.Time.Before(o.End) {
				previous = false
				continue
			}
			x, y := xpos(b.Time.Add(o.Period/2)), ypos(value)
			r := '·'
			if o.ASCII {
				r = '.'
			}
			if previous {
				segment(px, py, x, y, r, 3)
			}
			put(x, y, r, 3)
			px, py, previous = x, y, true
		}
	}
	type tag struct {
		text string
		kind int
	}
	tags := make(map[int]tag)
	reference := func(value float64, label string, kind int) {
		y := ypos(value)
		for x := 0; x < w; x++ {
			put(x, y, dash, kind)
		}
		text := label + " " + metrics.Format(o.Metric, value)
		tags[y] = tag{text, kind}
	}
	if o.ShowLast && !empty {
		if o.Line && len(points) > 0 {
			reference(points[len(points)-1].Value, "C", 3)
		} else if len(bars) > 0 {
			b := bars[len(bars)-1]
			kind := 1
			if b.Close < b.Open {
				kind = 2
			}
			reference(b.Close, "C", kind)
		}
	}
	cursorX := -1
	if !o.Cursor.IsZero() && !o.Cursor.Before(o.Start) && o.Cursor.Before(o.End) {
		cursorX = xpos(o.Cursor.Add(o.Period / 2))
		for y := 0; y < h; y++ {
			r := '┊'
			if o.ASCII {
				r = ':'
			}
			put(cursorX, y, r, 3)
		}
		for _, b := range bars {
			if b.Time.Equal(o.Cursor) {
				reference(b.Close, ">", 3)
				break
			}
		}
	}
	if o.Line {
		maxGap := max(o.Gap, span/time.Duration(max(1, w))*3)
		if maxGap <= 0 {
			maxGap = 3 * time.Second
		}
		for i, p := range points {
			if p.Time.Before(o.Start) || p.Time.After(o.End) {
				continue
			}
			x, y := xpos(p.Time), ypos(p.Value)
			if i > 0 && p.Time.Sub(points[i-1].Time) <= maxGap && !points[i-1].Time.Before(o.Start) {
				segment(xpos(points[i-1].Time), ypos(points[i-1].Value), x, y, dot, 3)
			}
			put(x, y, dot, 3)
		}
	} else {
		bodyWidth := 1
		if float64(o.Period)/float64(span)*float64(w) >= 4 {
			bodyWidth = 3
		}
		for _, b := range bars {
			if b.Time.Before(o.Start) || !b.Time.Before(o.End) {
				continue
			}
			x := xpos(b.Time.Add(o.Period / 2))
			kind := 1
			if b.Close < b.Open {
				kind = 2
			}
			for y := ypos(b.High); y <= ypos(b.Low); y++ {
				put(x, y, vertical, kind)
			}
			top, bottom := pixelY(math.Max(b.Open, b.Close)), pixelY(math.Min(b.Open, b.Close))
			half := bodyWidth / 2
			if b.Open == b.Close {
				for dx := -half; dx <= half; dx++ {
					r := horizontal
					if dx == 0 {
						r = '┼'
						if o.ASCII {
							r = '+'
						}
					}
					put(x+dx, top/2, r, kind)
				}
				continue
			}
			if top == bottom {
				r := '▀'
				if top%2 == 1 {
					r = '▄'
				}
				if o.ASCII {
					r = '='
				}
				for dx := -half; dx <= half; dx++ {
					put(x+dx, top/2, r, kind)
				}
				continue
			}
			// Hollow rising candles and solid falling candles remain distinct without color.
			if b.Close > b.Open && bodyWidth == 3 && bottom/2 > top/2 {
				for y := top / 2; y <= bottom/2; y++ {
					if y == top/2 || y == bottom/2 {
						left, right := '┌', '┐'
						if y == bottom/2 {
							left, right = '└', '┘'
						}
						if o.ASCII {
							left, right = '+', '+'
						}
						put(x-1, y, left, kind)
						put(x, y, horizontal, kind)
						put(x+1, y, right, kind)
					} else {
						put(x-1, y, vertical, kind)
						put(x, y, ' ', kind)
						put(x+1, y, vertical, kind)
					}
				}
			} else {
				for y := top / 2; y <= bottom/2; y++ {
					r := '█'
					if y == top/2 && top%2 == 1 {
						r = '▄'
					}
					if y == bottom/2 && bottom%2 == 0 {
						r = '▀'
					}
					if o.ASCII {
						r = '#'
					}
					for dx := -half; dx <= half; dx++ {
						put(x+dx, y, r, kind)
					}
				}
			}
		}
	}
	colors := []string{o.Grid, o.Up, o.Down, o.Accent, o.Muted}
	defaults := []string{"#334155", "#f87171", "#4ade80", "#60a5fa", "#94a3b8"}
	styles := make([]lipgloss.Style, len(colors))
	for i, c := range colors {
		if c == "" {
			c = defaults[i]
		}
		styles[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(c))
	}
	paint := func(s string, kind int) string {
		if o.NoColor {
			return s
		}
		return styles[kind].Render(s)
	}
	lines := make([]string, 0, o.Height)
	for y, row := range canvas {
		label := "       "
		if value, ok := ticks[y]; ok {
			label = fmt.Sprintf("%7s", axisValue(value))
		}
		var line strings.Builder
		line.WriteString(paint(label+" "+string(vertical), 4))
		for x := 0; x < w; {
			end := x + 1
			for end < w && row[end].kind == row[x].kind {
				end++
			}
			var run strings.Builder
			for _, c := range row[x:end] {
				run.WriteRune(c.r)
			}
			line.WriteString(paint(run.String(), row[x].kind))
			x = end
		}
		if t, ok := tags[y]; ok {
			line.WriteString(paint(" "+ansi.Truncate(t.text, valueWidth-1, ""), t.kind))
		}
		lines = append(lines, line.String())
	}
	axisLine := []rune(strings.Repeat(string(horizontal), w))
	if cursorX >= 0 {
		axisLine[cursorX] = '┴'
		if o.ASCII {
			axisLine[cursorX] = '+'
		}
	}
	lines = append(lines, paint(strings.Repeat(" ", axisWidth)+string(axisLine), 4))
	labels := []rune(strings.Repeat(" ", o.Width))
	format := "15:04"
	if o.Period < time.Minute {
		format = "15:04:05"
	}
	if span >= 24*time.Hour {
		format = "01/02"
	}
	n := max(2, min(5, w/(len(o.Start.Local().Format(format))+4)))
	for i := 0; i < n; i++ {
		t := o.Start.Add(time.Duration(float64(span) * float64(i) / float64(n-1)))
		label := t.Local().Format(format)
		pos := axisWidth + int(float64(w-1)*float64(i)/float64(n-1)) - len(label)/2
		pos = max(axisWidth, min(axisWidth+w-len(label), pos))
		copy(labels[pos:], []rune(label))
	}
	lines = append(lines, paint(string(labels), 4))
	if empty {
		msg := "No samples in this window"
		if o.Line {
			msg = "No raw samples (retention / gap)"
		} else if o.Period < time.Minute {
			msg = "No second samples (raw history / gap)"
		}
		msg = ansi.Truncate(msg, o.Width, "")
		lines[h/2] = paint(strings.Repeat(" ", max(0, (o.Width-ansi.StringWidth(msg))/2))+msg, 4)
	}
	return strings.Join(lines, "\n")
}

func axisValue(v float64) string {
	switch {
	case v > 0 && v < .001:
		return fmt.Sprintf("%.1e", v)
	case v > 0 && v < 1:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", v), "0"), ".")
	case v >= 1e12:
		return fmt.Sprintf("%.1e", v)
	case v >= 1e9:
		return fmt.Sprintf("%.1fG", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.1fM", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%.1fk", v/1e3)
	default:
		return fmt.Sprintf("%.1f", v)
	}
}
func fit(s string, w, h int) string {
	lines := make([]string, h)
	lines[0] = ansi.Truncate(s, w, "")
	return strings.Join(lines, "\n")
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
