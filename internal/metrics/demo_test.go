package metrics

import (
	"testing"
	"time"
)

func TestDemoVariesInsideOneSecond(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	first := DemoAt(now)
	second := DemoAt(now.Add(250 * time.Millisecond))
	if first.Values[CPU] == second.Values[CPU] {
		t.Fatal("subsecond reads are identical; 1s candles cannot have a body")
	}
	if DemoAt(now).Values != first.Values {
		t.Fatal("demo is not deterministic")
	}
}
