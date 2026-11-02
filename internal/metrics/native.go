package metrics

import (
	"errors"
	"fmt"
	"math"
	"time"
)

var errUnsupportedMetric = errors.New("metric unavailable on this platform")

type nativeSnapshot struct {
	cpuTotal, cpuIdle                                 float64
	memoryTotal, memoryAvailable, swapTotal, swapFree uint64
	uptime                                            float64
	load                                              [3]float64
	disk, net                                         map[string]counters
	loopbacks                                         map[string]bool
	diskErr, netErr, swapErr, loadErr, uptimeErr      error
}

type nativeCollector struct {
	read                    func() (nativeSnapshot, error)
	interfaces, disks       map[string]bool
	previous, diskAt, netAt time.Time
	cpuTotal, cpuIdle       float64
	disk, net               map[string]counters
	now                     func() time.Time
}

func newNativeCollector(read func() (nativeSnapshot, error), interfaces, disks []string) *nativeCollector {
	set := func(values []string) map[string]bool {
		m := make(map[string]bool)
		for _, v := range values {
			m[v] = true
		}
		return m
	}
	return &nativeCollector{read: read, interfaces: set(interfaces), disks: set(disks), now: time.Now}
}

func selectNativeCounters(values map[string]counters, filter, excluded map[string]bool) map[string]counters {
	out := make(map[string]counters)
	for name, value := range values {
		if len(filter) > 0 {
			if !filter[name] {
				continue
			}
		} else if excluded[name] {
			continue
		}
		out[name] = value
	}
	return out
}

func (c *nativeCollector) Sample() (Sample, error) {
	s := Sample{Time: c.now()}
	raw, err := c.read()
	if err != nil {
		return s, err
	}
	if raw.memoryTotal == 0 || math.IsNaN(raw.cpuTotal) || math.IsInf(raw.cpuTotal, 0) || math.IsNaN(raw.cpuIdle) || math.IsInf(raw.cpuIdle, 0) {
		return s, fmt.Errorf("invalid native CPU/memory counters")
	}
	s.MemoryTotal, s.SwapTotal, s.Uptime = raw.memoryTotal, raw.swapTotal, raw.uptime
	s.Values[Memory] = clamp(100 * (1 - float64(raw.memoryAvailable)/float64(raw.memoryTotal)))
	s.Unavailable[CPU] = true
	if !c.previous.IsZero() && raw.cpuTotal > c.cpuTotal && raw.cpuIdle >= c.cpuIdle {
		s.Values[CPU] = clamp(100 * (1 - (raw.cpuIdle-c.cpuIdle)/(raw.cpuTotal-c.cpuTotal)))
		s.Unavailable[CPU] = false
	}
	warn := func(name string, e error) {
		if e != nil && !errors.Is(e, errUnsupportedMetric) {
			s.Warnings = append(s.Warnings, name+": "+e.Error())
		}
	}
	if raw.swapErr != nil {
		s.Unavailable[Swap] = true
		warn("swap", raw.swapErr)
	} else if raw.swapTotal > 0 {
		s.Values[Swap] = clamp(100 * (1 - float64(raw.swapFree)/float64(raw.swapTotal)))
	}
	for i := range raw.load {
		if raw.loadErr != nil {
			s.Unavailable[Load1+i] = true
		} else {
			s.Values[Load1+i] = raw.load[i]
		}
	}
	warn("load", raw.loadErr)
	warn("uptime", raw.uptimeErr)
	update := func(values map[string]counters, e error, previous *map[string]counters, at *time.Time, read, write int) {
		s.Unavailable[read], s.Unavailable[write] = true, true
		if e != nil {
			warn(All[read].Key, e)
			return
		}
		if !at.IsZero() && s.Time.After(*at) {
			r, w, validR, validW := rates(values, *previous, s.Time.Sub(*at).Seconds())
			s.Values[read], s.Values[write] = r, w
			s.Unavailable[read], s.Unavailable[write] = !validR, !validW
			if read == NetRX && len(c.interfaces) == 0 && len(values) == 0 {
				s.Unavailable[read], s.Unavailable[write] = false, false
			}
		}
		*previous = values
		*at = s.Time
	}
	disks := selectNativeCounters(raw.disk, c.disks, nil)
	nets := selectNativeCounters(raw.net, c.interfaces, raw.loopbacks)
	update(disks, raw.diskErr, &c.disk, &c.diskAt, DiskRead, DiskWrite)
	update(nets, raw.netErr, &c.net, &c.netAt, NetRX, NetTX)
	if raw.diskErr == nil && len(disks) == 0 {
		s.Warnings = append(s.Warnings, "disk I/O: no selected counters available")
	}
	if raw.netErr == nil && len(c.interfaces) > 0 && len(nets) == 0 {
		s.Warnings = append(s.Warnings, "network: selected interfaces not found")
	}
	c.previous, c.cpuTotal, c.cpuIdle = s.Time, raw.cpuTotal, raw.cpuIdle
	return s, nil
}
