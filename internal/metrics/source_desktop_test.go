//go:build darwin || windows

package metrics

import (
	"testing"

	"github.com/shirou/gopsutil/v4/cpu"
)

func TestNativeCPUAccounting(t *testing.T) {
	// Windows System already contains IRQ time. gopsutil exposes Irq as
	// a separate diagnostic, so it must not be counted a second time.
	times := cpu.TimesStat{User: 10, System: 10, Idle: 80, Irq: 10}
	total, idle := nativeCPUTimes("windows", times)
	if total != 100 || idle != 80 {
		t.Fatalf("Windows CPU must be 20%% busy: total=%v idle=%v", total, idle)
	}
	times = cpu.TimesStat{User: 10, System: 15, Idle: 70, Nice: 5}
	total, idle = nativeCPUTimes("darwin", times)
	if total != 100 || idle != 70 {
		t.Fatalf("Darwin CPU must include Nice: total=%v idle=%v", total, idle)
	}
}
