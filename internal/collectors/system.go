package collectors

import (
    "github.com/shirou/gopsutil/v3/host"
    "time"
)

// OSInfo to match the structure
type OSInfo struct {
    Manufacturer      string
    Model             string
    Caption           string
    Version           string
    BuildNumber       string
    LogicalProcessors uint64
}

// ThermalZoneTemp ...
type ThermalZoneTemp struct {
    Instance    string
    TempCelsius float64
}

// EventLogStats ...
type EventLogStats struct {
    ErrorCount       uint64
    WarningCount     uint64
    InformationCount uint64
    OtherCount       uint64
}

// GetUptime returns system uptime in seconds
func GetUptime() uint64 {
    up, err := host.Uptime()
    if err != nil {
        return 0
    }
    return up
}

// GetProcessCount returns the total number of processes
func GetProcessCount() uint64 {
    info, err := host.ProcessCount()
    if err != nil {
        return 0
    }
    return info
}

// GetOSInfo returns placeholder info for cross-platform, with better detail on Windows in windows_only.go
func GetOSInfo() OSInfo {
    info := OSInfo{}
    hi, err := host.Info()
    if err == nil {
        info.Caption = hi.Platform
        info.Version = hi.PlatformVersion
        info.BuildNumber = hi.KernelVersion
    }

    // For cross-platform, there's no direct "Manufacturer" or "Model" from gopsutil.
    // On Windows, we fill this from WMI in windows_only.go
    // On other OSes, just return placeholders:
    info.Manufacturer = "Unknown"
    info.Model = "Unknown"

    // LogicalProcessors
    // host.Info() does not return CPU count. We can get that from CPU counts:
    cpus, err := host.CPUCounts(true)
    if err == nil {
        info.LogicalProcessors = uint64(cpus)
    }

    return info
}

// GetThermalZoneTemps returns empty for cross-platform by default.
// On Linux, you could use host.SensorsTemperatures() if permitted.
// Windows typically doesn't provide these via gopsutil. We'll see windows_only.go for specialized code.
func GetThermalZoneTemps() []ThermalZoneTemp {
    return []ThermalZoneTemp{}
}

// For demonstration, do stubs for pagefile usage, services, event logs (non-Windows):
func GetPageFileUsage() []PageFileUsage {
    return nil
}

func GetServices() []ServiceInfo {
    return nil
}

func GetEventLogStats() EventLogStats {
    return EventLogStats{}
}
