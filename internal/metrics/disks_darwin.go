//go:build darwin

package metrics

import "github.com/shirou/gopsutil/v4/disk"

func readNativeDisks() (map[string]counters, error) {
	values, err := disk.IOCounters()
	if err != nil {
		return nil, err
	}
	out := make(map[string]counters, len(values))
	for name, v := range values {
		out[name] = counters{v.ReadBytes, v.WriteBytes}
	}
	return out, nil
}
