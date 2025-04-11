package collectors

import (
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
)

type VolumeMetrics struct {
	DriveLetter     string
	FileSystemLabel string
	SizeBytes       uint64
	FreeBytes       uint64
}

func GetVolumeMetrics() []VolumeMetrics {
	var results []VolumeMetrics

	partitions, err := disk.Partitions(false)
	if err != nil {
		return results
	}

	for _, part := range partitions {
		usage, err := disk.Usage(part.Mountpoint)
		if err != nil || usage == nil {
			continue
		}

		driveLetter := part.Device
		if runtime.GOOS == "windows" {
			if idx := strings.Index(driveLetter, ":"); idx != -1 {
				driveLetter = driveLetter[:idx+1]
			}
		}

		results = append(results, VolumeMetrics{
			DriveLetter:     driveLetter,
			FileSystemLabel: part.Fstype,
			SizeBytes:       usage.Total,
			FreeBytes:       usage.Free,
		})
	}

	return results
}
