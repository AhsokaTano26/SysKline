package metrics

import (
	"math"
	"time"
)

// Demo is deterministic synthetic data, never a substitute for Linux metrics.
type Demo struct{}

func (Demo) Sample() (Sample, error) { return DemoAt(time.Now()), nil }

func DemoAt(t time.Time) Sample {
	x := float64(t.Unix()) + float64(t.Nanosecond())/1e9
	wave := func(offset float64) float64 {
		return math.Sin(x/113+offset)*.23 + math.Sin(x/19+offset)*.13 + math.Sin(x/1607+offset)*.17 + math.Sin(x*2.3+offset)*.045 + math.Sin(x*7.1+offset)*.025
	}
	s := Sample{Time: t, MemoryTotal: 32 << 30, SwapTotal: 8 << 30, Uptime: 86400*12 + x - math.Floor(x/86400)*86400}
	s.Values = [Count]float64{clamp(46 + 80*wave(0)), clamp(62 + 24*wave(2)), clamp(9 + 12*wave(4)), math.Max(0, 4e6+7e6*wave(1)), math.Max(0, 2e6+4e6*wave(3)), math.Max(0, 8e6+13e6*wave(2)), math.Max(0, 2e6+3e6*wave(4)), math.Max(0, 2.4+4*wave(0)), math.Max(0, 2.1+2*wave(1)), math.Max(0, 1.8+wave(2))}
	return s
}
