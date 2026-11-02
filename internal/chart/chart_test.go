package chart

import (
	"strings"
	"testing"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/metrics"
	"github.com/charmbracelet/x/ansi"
)

func TestCandleWicksBodiesAndBounds(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := []metrics.Candle{{Time: now, Open: 30, High: 90, Low: 10, Close: 70, Count: 2}, {Time: now.Add(2 * time.Minute), Open: 60, High: 80, Low: 20, Close: 40, Count: 2}}
	for _, ascii := range []bool{false, true} {
		o := Options{Width: 70, Height: 14, Start: now, End: now.Add(4 * time.Minute), Period: time.Minute, ASCII: ascii, NoColor: true, Cursor: now.Add(2 * time.Minute)}
		v := Render(bars, nil, o)
		lines := strings.Split(v, "\n")
		if len(lines) != 14 {
			t.Fatalf("height %d", len(lines))
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > 70 {
				t.Fatalf("overflow %d: %q", ansi.StringWidth(line), line)
			}
		}
		if ascii {
			for _, r := range v {
				if r > 127 {
					t.Fatal("non-ASCII output")
				}
			}
			if !strings.Contains(v, "#") || !strings.Contains(v, "|") {
				t.Fatal("missing body or wick")
			}
		} else if !strings.Contains(v, "█") || !strings.Contains(v, "│") {
			t.Fatalf("missing body or wick: %s", v)
		}
	}
}

func TestChartSmallEmptyFlatAndSparseLine(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	for _, size := range [][2]int{{0, 0}, {1, 1}, {12, 3}, {40, 8}, {100, 20}} {
		for _, bars := range [][]metrics.Candle{nil, {{Time: now, Open: 10, High: 10, Low: 10, Close: 10}}} {
			v := Render(bars, nil, Options{Width: size[0], Height: size[1], Start: now, End: now.Add(time.Minute), Period: time.Minute, NoColor: true})
			if size[1] > 0 && len(strings.Split(v, "\n")) > size[1] {
				t.Fatal("height overflow")
			}
			for _, line := range strings.Split(v, "\n") {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("width overflow")
				}
			}
		}
	}
	o := Options{Width: 70, Height: 12, Start: now, End: now.Add(10 * time.Minute), Period: time.Minute, Line: true, NoColor: true, ASCII: true}
	v := Render(nil, []metrics.Point{{Time: now, Value: 10}, {Time: now.Add(10 * time.Minute), Value: 90}}, o)
	// A ten-minute sampling gap must not be drawn as a continuous trend.
	if strings.Count(v, "*") > 3 {
		t.Fatalf("connected data gap: %s", v)
	}
}

func TestCandlesticksHaveOutlinedBodiesDojiAndSecondLabels(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
	bars := []metrics.Candle{{Time: start, Open: 30, High: 90, Low: 10, Close: 70}, {Time: start.Add(5 * time.Second), Open: 70, High: 90, Low: 10, Close: 30}, {Time: start.Add(10 * time.Second), Open: 50, High: 80, Low: 20, Close: 50}}
	o := Options{Width: 80, Height: 20, Start: start, End: start.Add(20 * time.Second), Period: 5 * time.Second, NoColor: true}
	v := Render(bars, nil, o)
	if !strings.Contains(v, "┌─┐") || !strings.Contains(v, "███") || !strings.Contains(v, "─┼─") {
		t.Fatalf("not distinct candle bodies/doji:\n%s", v)
	}
	if !strings.Contains(v, "12:00:00") || !strings.Contains(v, "12:00:20") {
		t.Fatal("second time labels missing", v)
	}
	o.ASCII = true
	v = Render(bars, nil, o)
	if !strings.Contains(v, "+-+") || !strings.Contains(v, "###") || !strings.Contains(v, "-+-") {
		t.Fatalf("ASCII candle geometry missing:\n%s", v)
	}
}

func TestLastValueAndCursorTags(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
	bars := []metrics.Candle{{Time: start, Open: 30, High: 90, Low: 10, Close: 70}, {Time: start.Add(5 * time.Second), Open: 70, High: 90, Low: 10, Close: 40}}
	v := Render(bars, nil, Options{Width: 80, Height: 20, Start: start, End: start.Add(10 * time.Second), Period: 5 * time.Second, NoColor: true, ShowLast: true, Cursor: start, Metric: metrics.CPU})
	if !strings.Contains(v, "C 40.0%") || !strings.Contains(v, "> 70.0%") {
		t.Fatal("close/crosshair value tags missing", v)
	}
}

func TestMovingAverageDoesNotBridgeMissingPeriods(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
	var bars []metrics.Candle
	for i := 0; i < 5; i++ {
		bars = append(bars, metrics.Candle{Time: start.Add(time.Duration(i) * time.Second), Close: float64((i + 1) * 10)})
	}
	v, ok := MovingAverage(bars, 4, 5, time.Second)
	if !ok || v != 30 {
		t.Fatalf("MA5 got %v %v", v, ok)
	}
	bars[2].Time = bars[2].Time.Add(time.Second)
	if _, ok := MovingAverage(bars, 4, 5, time.Second); ok {
		t.Fatal("moving average bridged a missing period")
	}
}

func TestAxisUsesRoundReadableTicks(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
	bars := []metrics.Candle{{Time: start, Open: 40, High: 67, Low: 33, Close: 60}}
	v := Render(bars, nil, Options{Width: 80, Height: 20, Start: start, End: start.Add(5 * time.Second), Period: 5 * time.Second, NoColor: true})
	if !strings.Contains(v, "   70.0") || !strings.Contains(v, "   30.0") || !strings.Contains(v, "   50.0") {
		t.Fatalf("non-round axis:\n%s", v)
	}
}

func TestHistoryOutsideViewportDoesNotStretchCandleScale(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
	bars := []metrics.Candle{{Time: start.Add(-time.Second), Open: 1000, High: 1200, Low: 900, Close: 1000}, {Time: start, Open: 40, High: 60, Low: 30, Close: 50}}
	v := Render(bars, nil, Options{Width: 80, Height: 20, Start: start, End: start.Add(time.Second), Period: time.Second, NoColor: true})
	if strings.Contains(v, "1.2k") || strings.Contains(v, "500.0") {
		t.Fatal("invisible history distorted visible scale", v)
	}
}

func TestSmallNonzeroBodyIsNotShownAsDoji(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
	bars := []metrics.Candle{{Time: start, Open: 50, High: 100, Low: 0, Close: 50.01}}
	v := Render(bars, nil, Options{Width: 80, Height: 10, Start: start, End: start.Add(time.Second), Period: time.Second, NoColor: true})
	if strings.Contains(v, "─┼─") {
		t.Fatal("nonzero body was rendered as a doji", v)
	}
}

func TestSmallValuesHaveDistinctAxisTicks(t *testing.T) {
	start := time.Now().Truncate(time.Second)
	bars := []metrics.Candle{{Time: start, Open: .02, High: .05, Low: .01, Close: .04}}
	v := Render(bars, nil, Options{Width: 80, Height: 20, Start: start, End: start.Add(time.Second), Period: time.Second, NoColor: true})
	if !strings.Contains(v, "   0.02") || !strings.Contains(v, "   0.04") {
		t.Fatal("small tick values collapse to 0.0", v)
	}
}

func TestLastRateTagKeepsUnitComplete(t *testing.T) {
	start := time.Now().Truncate(time.Second)
	bars := []metrics.Candle{{Time: start, Open: 10e6, High: 13e6, Low: 9e6, Close: 12e6}}
	v := Render(bars, nil, Options{Width: 57, Height: 8, Start: start, End: start.Add(time.Second), Period: time.Second, Metric: metrics.NetRX, ShowLast: true, NoColor: true})
	if !strings.Contains(v, "C 11.4MiB/s") {
		t.Fatal("rate tag dropped part of its unit", v)
	}
}
