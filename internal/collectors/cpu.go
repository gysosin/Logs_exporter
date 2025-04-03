package collectors

import (
    "github.com/shirou/gopsutil/v3/cpu"
    "github.com/shirou/gopsutil/v3/process"
    "time"
)

// PerProcessCPU holds a process name and its CPU usage in percent
type PerProcessCPU struct {
    Name      string
    CPUPercent float64
}

// GetCPUUsagePercent returns total CPU usage (system-wide) as a percentage.
func GetCPUUsagePercent() float64 {
    // On many systems, cpu.Percent(0, false) returns usage over a short timeslice.
    // It's often recommended to do an average over a small interval:
    percents, err := cpu.Percent(time.Millisecond*200, false)
    if err != nil || len(percents) == 0 {
        return 0
    }
    return percents[0]
}

// GetPerProcessCPU returns CPU usage per process as a slice
func GetPerProcessCPU() []PerProcessCPU {
    var results []PerProcessCPU

    procs, err := process.Processes()
    if err != nil {
        return results
    }

    // We need two samples to get an instantaneous CPU usage. For shortness, do one quick sample.
    // For more accurate usage, you'd sample, sleep a bit, then sample again. 
    // We'll just do a single sampling approach here for demonstration.
    for _, p := range procs {
        name, _ := p.Name()
        cpuPercent, err := p.CPUPercent()
        if err == nil {
            results = append(results, PerProcessCPU{
                Name:      name,
                CPUPercent: cpuPercent,
            })
        }
    }
    return results
}
