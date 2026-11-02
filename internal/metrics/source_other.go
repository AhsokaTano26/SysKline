//go:build !linux && !darwin && !windows

package metrics

import (
	"fmt"
	"runtime"
)

func NewSource(interfaces, disks []string) (Source, error) {
	return nil, fmt.Errorf("live metrics are not supported on %s", runtime.GOOS)
}
