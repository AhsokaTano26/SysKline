//go:build windows

package metrics

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows fixed-volume counters use read-only handles and never modify the
// registry or run diskperf. Unsupported/disabled counters are reported as n/a.
type diskPerformance struct {
	bytesRead, bytesWritten, readTime, writeTime, idleTime int64
	readCount, writeCount, queueDepth, splitCount          uint32
	queryTime                                              int64
	deviceNumber                                           uint32
	managerName                                            [8]uint16
}

func readNativeDisks() (map[string]counters, error) {
	buffer := make([]uint16, 512)
	length, err := windows.GetLogicalDriveStrings(uint32(len(buffer)), &buffer[0])
	if err != nil {
		return nil, err
	}
	if length >= uint32(len(buffer)) {
		return nil, fmt.Errorf("drive list exceeds buffer")
	}
	out := make(map[string]counters)
	var failures []error
	for _, letter := range buffer[:length] {
		if letter < 'A' || letter > 'Z' {
			continue
		}
		name := string(rune(letter)) + ":"
		path, _ := windows.UTF16PtrFromString(name + `\`)
		if windows.GetDriveType(path) != windows.DRIVE_FIXED {
			continue
		}
		device, _ := windows.UTF16PtrFromString(`\\.\` + name)
		h, e := windows.CreateFile(device, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
		if e != nil {
			failures = append(failures, fmt.Errorf("%s: %w", name, e))
			continue
		}
		var value diskPerformance
		var returned uint32
		const ioctlDiskPerformance = 0x70020
		e = windows.DeviceIoControl(h, ioctlDiskPerformance, nil, 0, (*byte)(unsafe.Pointer(&value)), uint32(unsafe.Sizeof(value)), &returned, nil)
		windows.CloseHandle(h)
		if e != nil {
			failures = append(failures, fmt.Errorf("%s: %w", name, e))
			continue
		}
		if value.bytesRead < 0 || value.bytesWritten < 0 {
			failures = append(failures, fmt.Errorf("%s: invalid disk counters", name))
			continue
		}
		out[name] = counters{uint64(value.bytesRead), uint64(value.bytesWritten)}
	}
	if len(out) == 0 && len(failures) > 0 {
		return nil, errors.Join(failures...)
	}
	return out, nil
}
