package collectors

import (
    "github.com/shirou/gopsutil/v3/disk"
)

// DiskMetrics holds usage stats for a partition
type DiskMetrics struct {
    Device string
    Total  uint64
    Used   uint64
    Free   uint64
}

// GetDiskMetrics returns disk usage for each partition
func GetDiskMetrics() []DiskMetrics {
    var results []DiskMetrics
    partitions, err := disk.Partitions(false)
    if err != nil {
        return results
    }

    for _, part := range partitions {
        usage, err := disk.Usage(part.Mountpoint)
        if err == nil && usage != nil {
            results = append(results, DiskMetrics{
                Device: part.Device,
                Total:  usage.Total,
                Used:   usage.Used,
                Free:   usage.Free,
            })
        }
    }
    return results
}
