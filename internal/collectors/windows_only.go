//go:build windows
// +build windows

package collectors

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/process"
)

// GetUptime returns system uptime in seconds for Windows.
func GetUptime() uint64 {
	up, err := host.Uptime()
	if err != nil {
		return 0
	}
	return up
}

// GetProcessCount returns the total number of processes on Windows.
func GetProcessCount() uint64 {
	procs, err := process.Processes()
	if err != nil {
		return 0
	}
	return uint64(len(procs))
}

// GetOSInfo returns OS information for Windows.
func GetOSInfo() OSInfo {
	info := OSInfo{
		Manufacturer:      "Microsoft Corporation",
		Model:             "Virtual Machine",
		Caption:           "Windows 10 Enterprise",
		Version:           "10.0.19044",
		BuildNumber:       "19044",
		LogicalProcessors: 0,
	}
	// Get logical processor count.
	base := GetOSInfoNonWindows()
	info.LogicalProcessors = base.LogicalProcessors
	return info
}

// Helper function to get CPU count.
func GetOSInfoNonWindows() OSInfo {
	var info OSInfo
	if cpus, err := cpu.Counts(true); err == nil {
		info.LogicalProcessors = uint64(cpus)
	}
	return info
}

// GetThermalZoneTemps returns an example thermal zone temperature for Windows.
func GetThermalZoneTemps() []ThermalZoneTemp {
	return []ThermalZoneTemp{
		{Instance: "CPU0", TempCelsius: 50.0},
	}
}

// PageFileUsage holds page file usage data (Windows-only).
type PageFileUsage struct {
	PageFile string
	UsagePct float64
}

// ServiceInfo holds Windows service information.
type ServiceInfo struct {
	Name       string
	Display    string
	StateValue int
	StartValue int
}

// GetPageFileUsage returns example page file usage data.
func GetPageFileUsage() []PageFileUsage {
	return []PageFileUsage{
		{PageFile: "C:\\pagefile.sys", UsagePct: 15.0},
	}
}

// GetServices returns example Windows service data.
func GetServices() []ServiceInfo {
	return []ServiceInfo{
		{
			Name:       "Spooler",
			Display:    "Print Spooler",
			StateValue: 1,
			StartValue: 1,
		},
		{
			Name:       "W32Time",
			Display:    "Windows Time",
			StateValue: 0,
			StartValue: 0,
		},
	}
}

// GetEventLogStats returns example event log statistics.
func GetEventLogStats() EventLogStats {
	return EventLogStats{
		ErrorCount:       2,
		WarningCount:     5,
		InformationCount: 50,
		OtherCount:       1,
	}
}
