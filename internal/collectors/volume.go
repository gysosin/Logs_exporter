package collectors

import (
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
)

// VolumeMetrics holds volume information similar to Get-Volume in PowerShell.
type VolumeMetrics struct {
	DriveLetter     string
	FileSystemLabel string
	SizeBytes       uint64
	FreeBytes       uint64
}

// GetVolumeMetrics returns volume metrics for each partition.
func GetVolumeMetrics() []VolumeMetrics {
	var results []VolumeMetrics

	// Use disk.Partitions to list mount points.
	partitions, err := disk.Partitions(false)
	if err != nil {
		return results
	}

	for _, part := range partitions {
		usage, err := disk.Usage(part.Mountpoint)
		if err != nil || usage == nil {
			continue
		}

		// Attempt to extract drive letter from the device string.
		driveLetter := part.Device
		if idx := strings.Index(driveLetter, ":"); idx != -1 {
			driveLetter = driveLetter[:idx+1]
		}

		results = append(results, VolumeMetrics{
			DriveLetter:     driveLetter,
			FileSystemLabel: part.Fstype, // Using Fstype as a substitute for a label.
			SizeBytes:       usage.Total,
			FreeBytes:       usage.Free,
		})
	}

	return results
}
