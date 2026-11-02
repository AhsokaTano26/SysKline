package metrics

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixture(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectorRatesAndReset(t *testing.T) {
	root := t.TempDir()
	write := func(cpu, disk, net string) {
		fixture(t, root, "stat", cpu)
		fixture(t, root, "meminfo", "MemTotal: 1000 kB\nMemAvailable: 250 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n")
		fixture(t, root, "loadavg", "1.25 2.50 3.75 1/99 1\n")
		fixture(t, root, "uptime", "3600.00 100.00\n")
		fixture(t, root, "diskstats", disk)
		fixture(t, root, "net/dev", net)
	}
	write("cpu 100 0 100 700 100 0 0 0 20 0\n", "8 0 sda 1 0 100 0 1 0 200 0 0 0 0\n", "eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0\n")
	c := NewCollector(root, t.TempDir(), nil, nil)
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }
	first, err := c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if first.Values[CPU] != 0 || first.Values[Memory] != 75 || first.Values[Swap] != 0 {
		t.Fatalf("first: %+v", first)
	}
	now = now.Add(2 * time.Second)
	write("cpu 140 0 120 820 120 0 0 0 30 0\n", "8 0 sda 1 0 120 0 1 0 240 0 0 0 0\n", "eth0: 1400 0 0 0 0 0 0 0 2600 0 0 0 0 0 0 0\nlo: 999999 0 0 0 0 0 0 0 999999 0 0 0 0 0 0 0\n")
	s, err := c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(s.Values[CPU]-30) > .001 || s.Values[DiskRead] != 5120 || s.Values[DiskWrite] != 10240 || s.Values[NetRX] != 200 || s.Values[NetTX] != 300 || s.Values[Load15] != 3.75 {
		t.Fatalf("rates: %+v", s.Values)
	}
	now = now.Add(time.Second)
	write("cpu 1 0 1 10 0 0 0 0\n", "8 0 sda 1 0 1 0 1 0 1 0 0 0 0\n8 16 sdb 1 0 999999 0 1 0 999999 0 0 0 0\n", "eth0: 1 0 0 0 0 0 0 0 1 0 0 0 0 0 0 0\neth1: 999999 0 0 0 0 0 0 0 999999 0 0 0 0 0 0 0\n")
	s, err = c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{CPU, DiskRead, DiskWrite, NetRX, NetTX} {
		if s.Values[i] != 0 {
			t.Fatalf("reset/hotplug spike: %v", s.Values)
		}
	}
}

func TestCollectorFiltersAndOptionalSources(t *testing.T) {
	root := t.TempDir()
	fixture(t, root, "stat", "cpu 10 0 10 80 0 0 0 0\n")
	fixture(t, root, "meminfo", "MemTotal: 1000 kB\nMemFree: 100 kB\nBuffers: 100 kB\nCached: 100 kB\nSReclaimable: 50 kB\nShmem: 50 kB\n")
	c := NewCollector(root, t.TempDir(), []string{"eth0"}, []string{"sda"})
	s, err := c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if s.Values[Memory] != 70 || len(s.Warnings) == 0 {
		t.Fatalf("fallback or warnings: %+v", s)
	}
	if !s.Unavailable[DiskRead] || !s.Unavailable[NetRX] || !s.Unavailable[Load1] {
		t.Fatal("missing sources reported as actual zeros")
	}
	if _, err := NewCollector(t.TempDir(), t.TempDir(), nil, nil).Sample(); err == nil {
		t.Fatal("missing cpu must fail")
	}
}

func TestCounterSourceRecoveryUsesItsOwnElapsedTime(t *testing.T) {
	root := t.TempDir()
	fixture(t, root, "stat", "cpu 1 0 1 10 0 0 0 0\n")
	fixture(t, root, "meminfo", "MemTotal: 1000 kB\nMemAvailable: 500 kB\n")
	fixture(t, root, "net/dev", "eth0: 100 0 0 0 0 0 0 0 200 0 0 0 0 0 0 0\n")
	c := NewCollector(root, t.TempDir(), nil, nil)
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }
	if _, err := c.Sample(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "net/dev")); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	missing, err := c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if !missing.Unavailable[NetRX] {
		t.Fatal("unavailable network reported as zero")
	}
	now = now.Add(time.Second)
	fixture(t, root, "net/dev", "eth0: 300 0 0 0 0 0 0 0 600 0 0 0 0 0 0 0\n")
	s, err := c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if s.Unavailable[NetRX] || s.Values[NetRX] != 100 || s.Values[NetTX] != 200 {
		t.Fatalf("recovery delta wrong: %+v", s)
	}
}

func TestSelectedInterfaceAndWholeDiskExcludesPartitions(t *testing.T) {
	root, sys := t.TempDir(), t.TempDir()
	fixture(t, sys, "dev/block/8:1/partition", "1\n")
	fixture(t, root, "stat", "cpu 1 0 1 10 0 0 0 0\n")
	fixture(t, root, "meminfo", "MemTotal: 1000 kB\nMemAvailable: 500 kB\n")
	write := func(sectors, rx int) {
		fixture(t, root, "diskstats", fmt.Sprintf("8 0 sda 1 0 %d 0 1 0 0 0 0 0 0\n8 1 sda1 1 0 %d 0 1 0 0 0 0 0 0\n253 0 dm-0 1 0 %d 0 1 0 0 0 0 0 0\n", sectors, sectors, sectors))
		fixture(t, root, "net/dev", fmt.Sprintf("eth0: %d 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\neth1: %d 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n", rx, rx*10))
	}
	c := NewCollector(root, sys, []string{"eth0"}, nil)
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }
	write(100, 100)
	if _, err := c.Sample(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	write(110, 300)
	s, err := c.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if s.Values[DiskRead] != 5120 || s.Values[NetRX] != 200 {
		t.Fatalf("partition/interface double count: %+v", s.Values)
	}
}
