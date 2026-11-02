package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/metrics"
)

func TestOHLCOrderedRollupAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, v := range []struct {
		sec   int
		value float64
	}{{40, 20}, {10, 50}, {30, 5}, {70, 80}, {290, 30}, {300, 99}} {
		x := metrics.Sample{Time: base.Add(time.Duration(v.sec) * time.Second)}
		x.Values[0] = v.value
		if err := s.Insert(x); err != nil {
			t.Fatal(err)
		}
		if err := s.Insert(x); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	bars, err := s.Candles(0, time.Minute, base.Add(59*time.Second), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 1 || bars[0].Open != 50 || bars[0].Close != 20 || bars[0].High != 50 || bars[0].Low != 5 || bars[0].Count != 3 {
		t.Fatalf("minute bars: %+v", bars)
	}
	bars, err = s.Candles(0, 5*time.Minute, base.Add(10*time.Minute), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 2 || bars[0].Open != 50 || bars[0].Close != 30 || bars[0].High != 80 || bars[0].Low != 5 || bars[0].Count != 5 || bars[1].Open != 99 {
		t.Fatalf("rollup: %+v", bars)
	}
	if _, err := s.Candles(-1, time.Minute, base, 10); err == nil {
		t.Fatal("invalid metric")
	}
	if _, err := s.Candles(0, 2*time.Minute, base, 10); err == nil {
		t.Fatal("invalid period")
	}
}

func TestRetentionAndRawPoints(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	for _, age := range []time.Duration{10 * 24 * time.Hour, 2 * 24 * time.Hour, 0} {
		for i := 0; i < 20; i++ {
			x := metrics.Sample{Time: now.Add(-age).Add(time.Duration(i) * time.Second)}
			x.Values[0] = float64(i)
			if err := s.Insert(x); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.Cleanup(now, 1, 7); err != nil {
		t.Fatal(err)
	}
	p, err := s.Points(0, now.Add(-20*24*time.Hour), now.Add(time.Minute), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) > 5 || len(p) == 0 || p[0].Time.Before(now) {
		t.Fatalf("points: %+v", p)
	}
	bars, err := s.Candles(0, 24*time.Hour, now.Add(time.Hour), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 2 {
		t.Fatalf("retention bars: %+v", bars)
	}
	if bars[0].Time != now.Add(-2*24*time.Hour) {
		t.Fatalf("gap filled or time wrong: %+v", bars)
	}
}

func TestInvalidSampleDoesNotPartiallyPersist(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	x := metrics.Sample{Time: time.Now()}
	x.Values[3] = -1
	if err := s.Insert(x); err == nil {
		t.Fatal("negative metric accepted")
	}
	bars, err := s.Candles(0, time.Minute, time.Now(), 20)
	if err != nil || len(bars) != 0 {
		t.Fatalf("partial write: %+v %v", bars, err)
	}
}

func TestPointsSelectActualSamplesInBoundedTimeBins(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		x := metrics.Sample{Time: base.Add(time.Duration(i) * time.Second)}
		x.Values[0] = float64(i * i)
		if err := s.Insert(x); err != nil {
			t.Fatal(err)
		}
	}
	points, err := s.Points(0, base, base.Add(9*time.Second), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 || points[0].Value != 4 || points[1].Value != 25 || points[2].Value != 81 {
		t.Fatalf("expected actual last sample per bin, got %+v", points)
	}
}

func TestUnavailableSourceDoesNotCreateZeroCandle(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	x := metrics.Sample{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	x.Values[metrics.Memory] = 50
	x.Unavailable[metrics.NetRX] = true
	if err := s.Insert(x); err != nil {
		t.Fatal(err)
	}
	bars, err := s.Candles(metrics.NetRX, time.Minute, x.Time, 4)
	if err != nil || len(bars) != 0 {
		t.Fatalf("missing source became candle: %+v %v", bars, err)
	}
	points, err := s.Points(metrics.NetRX, x.Time, x.Time.Add(time.Minute), 4)
	if err != nil || len(points) != 0 {
		t.Fatalf("missing source became point: %+v %v", points, err)
	}
	bars, err = s.Candles(metrics.Memory, time.Minute, x.Time, 4)
	if err != nil || len(bars) != 1 || bars[0].Close != 50 {
		t.Fatal("valid source lost", bars, err)
	}
}

func TestPointBinEndsWithUnavailableMetricStaysGap(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	x := metrics.Sample{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	x.Values[metrics.NetRX] = 100
	if err := s.Insert(x); err != nil {
		t.Fatal(err)
	}
	x.Time = x.Time.Add(time.Second)
	x.Unavailable[metrics.NetRX] = true
	if err := s.Insert(x); err != nil {
		t.Fatal(err)
	}
	p, err := s.Points(metrics.NetRX, x.Time.Add(-time.Second), x.Time.Add(time.Second), 1)
	if err != nil || len(p) != 0 {
		t.Fatalf("carried old value through missing bin end: %+v %v", p, err)
	}
}

func TestLongPeriodBoundaries(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, seconds := range []int{0, 899, 900, 3599, 3600, 86399, 86400} {
		x := metrics.Sample{Time: base.Add(time.Duration(seconds) * time.Second)}
		x.Values[0] = float64((i + 1) * 10)
		if err := s.Insert(x); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		period time.Duration
		bars   int
		close  float64
		count  int64
	}{{15 * time.Minute, 6, 20, 2}, {time.Hour, 4, 40, 4}, {24 * time.Hour, 2, 60, 6}} {
		bars, err := s.Candles(0, tc.period, base.Add(24*time.Hour), 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(bars) != tc.bars || bars[0].Open != 10 || bars[0].Close != tc.close || bars[0].Count != tc.count || bars[len(bars)-1].Close != 70 {
			t.Fatalf("%s: %+v", tc.period, bars)
		}
	}
}

func TestSecondCandlesUseSubsecondSamplesAndPreserveGaps(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, p := range []struct {
		ms      int
		value   float64
		missing bool
	}{{750, 20, false}, {0, 40, false}, {250, 70, false}, {500, 10, false}, {1000, 80, true}, {2500, 30, false}, {5000, 90, false}, {15000, 50, false}} {
		x := metrics.Sample{Time: base.Add(time.Duration(p.ms) * time.Millisecond)}
		x.Values[0] = p.value
		x.Unavailable[0] = p.missing
		if err := s.Insert(x); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		period           time.Duration
		bars             int
		close, high, low float64
		count            int64
	}{{time.Second, 4, 20, 70, 10, 4}, {5 * time.Second, 3, 30, 70, 10, 5}, {15 * time.Second, 2, 90, 90, 10, 6}} {
		bars, err := s.Candles(0, tc.period, base.Add(15*time.Second), 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(bars) != tc.bars || bars[0].Open != 40 || bars[0].Close != tc.close || bars[0].High != tc.high || bars[0].Low != tc.low || bars[0].Count != tc.count {
			t.Fatalf("%s: %+v", tc.period, bars)
		}
	}
	if err := s.Cleanup(base.Add(2*24*time.Hour), 1, 7); err != nil {
		t.Fatal(err)
	}
	seconds, err := s.Candles(0, time.Second, base.Add(15*time.Second), 20)
	if err != nil || len(seconds) != 0 {
		t.Fatalf("seconds invented from minute history: %+v %v", seconds, err)
	}
	minutes, err := s.Candles(0, time.Minute, base, 4)
	if err != nil || len(minutes) != 1 || minutes[0].Count != 7 {
		t.Fatal("long history lost", minutes, err)
	}
}
