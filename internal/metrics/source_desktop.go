//go:build darwin || windows

package metrics

import (
	"fmt"
	stdnet "net"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
)

func NewSource(interfaces, disks []string) (Source, error) {
	return newNativeCollector(readNativeSnapshot, interfaces, disks), nil
}

func nativeCPUTimes(platform string, t cpu.TimesStat) (total, idle float64) {
	if platform == "windows" {
		// Windows kernel time includes interrupts; gopsutil System has only
		// Idle subtracted. Its separate Irq field overlaps with System.
		return t.User + t.System + t.Idle, t.Idle
	}
	return t.User + t.System + t.Idle + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal, t.Idle + t.Iowait
}

func readNativeSnapshot() (nativeSnapshot, error) {
	var s nativeSnapshot
	times, err := cpu.Times(false)
	if err != nil {
		return s, fmt.Errorf("CPU counters: %w", err)
	}
	if len(times) == 0 {
		return s, fmt.Errorf("no native CPU counters")
	}
	for _, t := range times {
		total, idle := nativeCPUTimes(runtime.GOOS, t)
		s.cpuTotal += total
		s.cpuIdle += idle
	}
	vm, err := mem.VirtualMemory()
	if err != nil {
		return s, fmt.Errorf("memory counters: %w", err)
	}
	s.memoryTotal, s.memoryAvailable = vm.Total, vm.Available
	swap, err := mem.SwapMemory()
	s.swapErr = err
	if err == nil {
		s.swapTotal, s.swapFree = swap.Total, swap.Free
	}
	s.uptime = 0
	up, err := host.Uptime()
	s.uptimeErr = err
	if err == nil {
		s.uptime = float64(up)
	}
	if runtime.GOOS == "windows" {
		s.loadErr = errUnsupportedMetric
	} else {
		avg, e := load.Avg()
		s.loadErr = e
		if e == nil {
			s.load = [3]float64{avg.Load1, avg.Load5, avg.Load15}
		}
	}
	s.disk, s.diskErr = readNativeDisks()
	nets, err := gnet.IOCounters(true)
	s.netErr = err
	s.net = make(map[string]counters)
	for _, n := range nets {
		s.net[n.Name] = counters{n.BytesRecv, n.BytesSent}
	}
	s.loopbacks = make(map[string]bool)
	if interfaces, e := stdnet.Interfaces(); e == nil {
		for _, i := range interfaces {
			if i.Flags&stdnet.FlagLoopback != 0 {
				s.loopbacks[i.Name] = true
			}
		}
	}
	for name := range s.net {
		if name == "lo" || name == "lo0" || strings.Contains(strings.ToLower(name), "loopback") {
			s.loopbacks[name] = true
		}
	}
	return s, nil
}
