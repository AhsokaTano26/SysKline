//go:build linux

package metrics

func NewSource(interfaces, disks []string) (Source, error) {
	return NewCollector("/proc", "/sys", interfaces, disks), nil
}
