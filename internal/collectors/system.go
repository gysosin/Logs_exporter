//go:build !windows
// +build !windows

package collectors

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/process"
)

// GetUptime returns system uptime in seconds.
func GetUptime() uint64 {
	up, err := host.Uptime()
	if err != nil {
		return 0
	}
	return up
}

// GetProcessCount returns the total number of processes.
func GetProcessCount() uint64 {
	procs, err := process.Processes()
	if err != nil {
		return 0
	}
	return uint64(len(procs))
}

// GetOSInfo returns placeholder OS information for non‑Windows.
func GetOSInfo() OSInfo {
	info := OSInfo{}
	hi, err := host.Info()
	if err == nil {
		info.Caption = hi.Platform
		info.Version = hi.PlatformVersion
		info.BuildNumber = hi.KernelVersion
	}
	// Use placeholders for fields not available cross‑platform.
	info.Manufacturer = "Unknown"
	info.Model = "Unknown"

	if cpus, err := cpu.Counts(true); err == nil {
		info.LogicalProcessors = uint64(cpus)
	}
	return info
}

// GetThermalZoneTemps returns an empty slice for non‑Windows.
func GetThermalZoneTemps() []ThermalZoneTemp {
	return []ThermalZoneTemp{}
}

// For non‑Windows, these stubs return nil.
func GetPageFileUsage() []PageFileUsage {
	return nil
}

func GetServices() []ServiceInfo {
	return nil
}

func GetEventLogStats() EventLogStats {
	return EventLogStats{}
}
