package metrics

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type counters struct{ read, write uint64 }
type cpuCounters struct{ total, idle uint64 }

type Collector struct {
	proc, sys         string
	interfaces, disks map[string]bool
	previous          time.Time
	cpu               cpuCounters
	disk, net         map[string]counters
	diskAt, netAt     time.Time
	now               func() time.Time
}

func NewCollector(proc, sys string, interfaces, disks []string) *Collector {
	set := func(ss []string) map[string]bool {
		m := make(map[string]bool)
		for _, s := range ss {
			m[s] = true
		}
		return m
	}
	return &Collector{proc: proc, sys: sys, interfaces: set(interfaces), disks: set(disks), now: time.Now}
}

func number(s string) uint64   { n, _ := strconv.ParseUint(s, 10, 64); return n }
func decimal(s string) float64 { n, _ := strconv.ParseFloat(s, 64); return n }
func clamp(v float64) float64  { return math.Max(0, math.Min(100, v)) }

func (c *Collector) read(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(c.proc, name))
	return string(b), err
}

func (c *Collector) Sample() (Sample, error) {
	s := Sample{Time: c.now()}
	stat, err := c.read("stat")
	if err != nil {
		return s, fmt.Errorf("read proc/stat: %w", err)
	}
	f := strings.Fields(strings.SplitN(stat, "\n", 2)[0])
	if len(f) < 5 || f[0] != "cpu" {
		return s, fmt.Errorf("invalid proc/stat CPU counters")
	}
	cur := cpuCounters{}
	// Guest and guest_nice (fields 9,10) are already included in user/nice.
	for i := 1; i < len(f) && i <= 8; i++ {
		v, err := strconv.ParseUint(f[i], 10, 64)
		if err != nil {
			return s, fmt.Errorf("invalid proc/stat counter: %w", err)
		}
		cur.total += v
	}
	cur.idle = number(f[4])
	if len(f) > 5 {
		cur.idle += number(f[5])
	}
	if !c.previous.IsZero() && cur.total > c.cpu.total && cur.idle >= c.cpu.idle {
		total, idle := cur.total-c.cpu.total, cur.idle-c.cpu.idle
		s.Values[CPU] = clamp(100 * (1 - float64(idle)/float64(total)))
	} else {
		s.Unavailable[CPU] = true
	}
	mem, err := c.read("meminfo")
	if err != nil {
		return s, fmt.Errorf("read proc/meminfo: %w", err)
	}
	m := map[string]uint64{}
	for _, line := range strings.Split(mem, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			m[strings.TrimSuffix(f[0], ":")] = number(f[1]) * 1024
		}
	}
	s.MemoryTotal, s.SwapTotal = m["MemTotal"], m["SwapTotal"]
	if s.MemoryTotal == 0 {
		return s, fmt.Errorf("invalid proc/meminfo MemTotal")
	}
	available, ok := m["MemAvailable"]
	if !ok {
		available = m["MemFree"] + m["Buffers"] + m["Cached"] + m["SReclaimable"]
		if available >= m["Shmem"] {
			available -= m["Shmem"]
		}
	}
	s.Values[Memory] = clamp(100 * (1 - float64(available)/float64(s.MemoryTotal)))
	if s.SwapTotal > 0 {
		s.Values[Swap] = clamp(100 * (1 - float64(m["SwapFree"])/float64(s.SwapTotal)))
	}
	optional := func(name string) (string, bool) {
		data, err := c.read(name)
		if err != nil {
			s.Warnings = append(s.Warnings, name+": "+err.Error())
		}
		return data, err == nil
	}
	loadData, _ := optional("loadavg")
	load := strings.Fields(loadData)
	if len(load) >= 3 {
		for i := 0; i < 3; i++ {
			v, err := strconv.ParseFloat(load[i], 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				s.Unavailable[Load1+i] = true
			} else {
				s.Values[Load1+i] = v
			}
		}
	} else {
		for i := Load1; i <= Load15; i++ {
			s.Unavailable[i] = true
		}
	}
	upData, _ := optional("uptime")
	up := strings.Fields(upData)
	if len(up) > 0 {
		s.Uptime = decimal(up[0])
	}
	disks := map[string]counters{}
	diskData, diskOK := optional("diskstats")
	for _, line := range strings.Split(diskData, "\n") {
		f := strings.Fields(line)
		if len(f) < 14 {
			continue
		}
		name := f[2]
		if len(c.disks) > 0 {
			if !c.disks[name] {
				continue
			}
		} else {
			if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "md") || strings.HasPrefix(name, "zram") {
				continue
			}
			if _, err := os.Stat(filepath.Join(c.sys, "dev/block", f[0]+":"+f[1], "partition")); err == nil {
				continue
			}
		}
		disks[name] = counters{number(f[5]) * 512, number(f[9]) * 512}
	}
	nets := map[string]counters{}
	netData, netOK := optional("net/dev")
	for _, line := range strings.Split(netData, "\n") {
		pair := strings.SplitN(line, ":", 2)
		if len(pair) != 2 {
			continue
		}
		name := strings.TrimSpace(pair[0])
		if len(c.interfaces) > 0 {
			if !c.interfaces[name] {
				continue
			}
		} else if name == "lo" {
			continue
		}
		f := strings.Fields(pair[1])
		if len(f) < 16 {
			continue
		}
		nets[name] = counters{number(f[0]), number(f[8])}
	}
	updateRates := func(current map[string]counters, ok bool, previous *map[string]counters, at *time.Time, read, write int) {
		s.Unavailable[read], s.Unavailable[write] = true, true
		elapsed := s.Time.Sub(*at).Seconds()
		if ok && !at.IsZero() && elapsed > 0 {
			r, w, validR, validW := rates(current, *previous, elapsed)
			s.Values[read], s.Values[write] = r, w
			s.Unavailable[read], s.Unavailable[write] = !validR, !validW
			// An empty default interface set represents no external network.
			if read == NetRX && len(c.interfaces) == 0 && len(current) == 0 {
				s.Unavailable[read], s.Unavailable[write] = false, false
			}
		}
		if ok {
			*previous = current
			*at = s.Time
		}
	}
	updateRates(disks, diskOK, &c.disk, &c.diskAt, DiskRead, DiskWrite)
	updateRates(nets, netOK, &c.net, &c.netAt, NetRX, NetTX)
	if diskOK && len(disks) == 0 {
		s.Warnings = append(s.Warnings, "diskstats: no selected whole-disk counters")
	}
	if netOK && len(c.interfaces) > 0 && len(nets) == 0 {
		s.Warnings = append(s.Warnings, "net/dev: selected interfaces not found")
	}
	c.previous, c.cpu = s.Time, cur
	return s, nil
}

func rates(current, previous map[string]counters, seconds float64) (float64, float64, bool, bool) {
	var read, write float64
	var validRead, validWrite bool
	for name, cur := range current {
		if prev, ok := previous[name]; ok {
			if cur.read >= prev.read {
				read += float64(cur.read-prev.read) / seconds
				validRead = true
			}
			if cur.write >= prev.write {
				write += float64(cur.write-prev.write) / seconds
				validWrite = true
			}
		}
	}
	return read, write, validRead, validWrite
}
