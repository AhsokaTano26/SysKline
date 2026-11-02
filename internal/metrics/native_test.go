package metrics

import (
	"errors"
	"testing"
	"time"
)

func TestNativeRatesMissingSourcesAndCounterRecovery(t *testing.T) {
	raw := nativeSnapshot{cpuTotal: 100, cpuIdle: 60, memoryTotal: 1000, memoryAvailable: 250, swapTotal: 400, swapFree: 300, uptime: 3600, load: [3]float64{1, 2, 3}, disk: map[string]counters{"disk0": {100, 200}}, net: map[string]counters{"Ethernet 2": {1000, 2000}, "Loopback": {999999, 999999}}, loopbacks: map[string]bool{"Loopback": true}}
	c := newNativeCollector(func() (nativeSnapshot, error) { return raw, nil }, []string{"Ethernet 2"}, []string{"disk0"})
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }
	first, err := c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if first.Values[Memory] != 75 || first.Values[Swap] != 25 || !first.Unavailable[CPU] || !first.Unavailable[NetRX] {
		t.Fatal("baseline", first)
	}
	now = now.Add(2 * time.Second)
	raw.cpuTotal = 120
	raw.cpuIdle = 70
	raw.disk = map[string]counters{"disk0": {300, 600}, "disk1": {999999, 999999}}
	raw.net = map[string]counters{"Ethernet 2": {1400, 2600}, "Loopback": {99999999, 99999999}}
	s, err := c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if s.Values[CPU] != 50 || s.Values[DiskRead] != 100 || s.Values[DiskWrite] != 200 || s.Values[NetRX] != 200 || s.Values[NetTX] != 300 || s.Values[Load15] != 3 {
		t.Fatal("wrong native rates", s)
	}
	now = now.Add(time.Second)
	raw.netErr = errors.New("network unavailable")
	raw.loadErr = errUnsupportedMetric
	s, err = c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if !s.Unavailable[NetRX] || !s.Unavailable[Load1] || len(s.Warnings) != 1 {
		t.Fatal("missing sources became zeros or unsupported load warned", s)
	}
	now = now.Add(time.Second)
	raw.netErr = nil
	raw.net = map[string]counters{"Ethernet 2": {1800, 3200}}
	s, err = c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if s.Values[NetRX] != 200 || s.Values[NetTX] != 300 {
		t.Fatal("recovery elapsed time wrong", s)
	}
	now = now.Add(time.Second)
	raw.net = map[string]counters{"Ethernet 2": {1, 1}}
	raw.cpuTotal = 1
	raw.cpuIdle = 1
	s, err = c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if !s.Unavailable[CPU] || !s.Unavailable[NetRX] {
		t.Fatal("reset produced a fake sample", s)
	}
}

func TestNativeSourceExcludesLoopbackByDefault(t *testing.T) {
	raw := nativeSnapshot{cpuTotal: 100, cpuIdle: 60, memoryTotal: 1000, memoryAvailable: 250, net: map[string]counters{"eth": {100, 200}, "lo0": {100, 200}}, loopbacks: map[string]bool{"lo0": true}}
	c := newNativeCollector(func() (nativeSnapshot, error) { return raw, nil }, nil, nil)
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }
	if _, err := c.Sample(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	raw.net = map[string]counters{"eth": {200, 400}, "lo0": {10000, 20000}}
	s, err := c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if s.Values[NetRX] != 100 || s.Values[NetTX] != 200 {
		t.Fatal("loopback counted", s.Values)
	}
}

func TestPlatformNativeSnapshot(t *testing.T) {
	source, err := NewSource(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Sample(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)
	s, err := source.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if s.Time.IsZero() || s.MemoryTotal == 0 || s.Unavailable[Memory] || s.Values[Memory] < 0 || s.Values[Memory] > 100 {
		t.Fatal("native snapshot invalid", s)
	}
	if !s.Unavailable[CPU] && (s.Values[CPU] < 0 || s.Values[CPU] > 100) {
		t.Fatal("CPU range invalid", s)
	}
}
