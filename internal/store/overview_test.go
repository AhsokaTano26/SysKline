package store

import (
	"testing"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/metrics"
)

func TestAllCandlesMatchSingleMetricWithGapsAndRetention(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, ms := range []int{750, 0, 250, 500, 2000, 60000, 61000, 300000} {
		x := metrics.Sample{Time: base.Add(time.Duration(ms) * time.Millisecond)}
		for i := range x.Values {
			x.Values[i] = float64(ms%97 + i + 1)
		}
		x.Unavailable[metrics.NetTX] = true
		if err := s.Insert(x); err != nil {
			t.Fatal(err)
		}
	}
	for _, period := range []time.Duration{time.Second, 5 * time.Second, 15 * time.Second, time.Minute, 5 * time.Minute, time.Hour, 24 * time.Hour} {
		batch, err := s.AllCandles(period, base.Add(5*time.Minute), 40)
		if err != nil {
			t.Fatal(err)
		}
		for metric := 0; metric < metrics.Count; metric++ {
			single, err := s.Candles(metric, period, base.Add(5*time.Minute), 40)
			if err != nil {
				t.Fatal(err)
			}
			if len(batch[metric]) != len(single) {
				t.Fatalf("%s metric %d bar count", period, metric)
			}
			for i := range single {
				if batch[metric][i] != single[i] {
					t.Fatalf("%s metric %d: %+v != %+v", period, metric, batch[metric][i], single[i])
				}
			}
		}
		if len(batch[metrics.NetTX]) != 0 {
			t.Fatal("unavailable metric became a zero candle")
		}
	}
	first, err := s.AllCandles(5*time.Second, base.Add(2*time.Second), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(first[metrics.CPU]) != 1 {
		t.Fatal("first real bucket missing", first[metrics.CPU])
	}
	b := first[metrics.CPU][0]
	if b.Open != 1 || b.High != 72 || b.Low != 1 || b.Close != 61 || b.Count != 5 {
		t.Fatalf("hand-calculated OHLC mismatch: %+v", b)
	}
	if err := s.Cleanup(base.Add(2*24*time.Hour), 1, 7); err != nil {
		t.Fatal(err)
	}
	seconds, err := s.AllCandles(time.Second, base.Add(5*time.Minute), 40)
	if err != nil || len(seconds[0]) != 0 {
		t.Fatal("seconds survived raw cleanup", err)
	}
	minutes, err := s.AllCandles(time.Minute, base.Add(5*time.Minute), 40)
	if err != nil || len(minutes[0]) != 3 {
		t.Fatal("minute history lost", minutes[0], err)
	}
	if _, err := s.AllCandles(2*time.Second, base, 40); err == nil {
		t.Fatal("unsupported period accepted")
	}
	if _, err := s.AllCandles(time.Minute, base, 0); err == nil {
		t.Fatal("invalid limit accepted")
	}
}

func TestAllPointsSelectSameIndexedBinsAndNullGaps(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		x := metrics.Sample{Time: base.Add(time.Duration(i) * time.Second)}
		for metric := range x.Values {
			x.Values[metric] = float64(i*i + metric)
		}
		x.Unavailable[metrics.NetRX] = i >= 6
		if err := s.Insert(x); err != nil {
			t.Fatal(err)
		}
	}
	batch, err := s.AllPoints(base, base.Add(9*time.Second), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch[0]) != 3 || batch[0][0].Value != 4 || batch[0][1].Value != 25 || batch[0][2].Value != 81 {
		t.Fatal("wrong bins", batch[0])
	}
	if len(batch[metrics.NetRX]) != 2 {
		t.Fatal("missing last bin was filled", batch[metrics.NetRX])
	}
	for metric := 0; metric < metrics.Count; metric++ {
		single, err := s.Points(metric, base, base.Add(9*time.Second), 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(single) != len(batch[metric]) {
			t.Fatal("point count mismatch")
		}
		for i := range single {
			if single[i] != batch[metric][i] {
				t.Fatalf("metric %d: point mismatch", metric)
			}
		}
	}
	if _, err := s.AllPoints(base, base, 3); err == nil {
		t.Fatal("zero span accepted")
	}
}
