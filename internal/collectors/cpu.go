package collectors

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/process"
)

// PerProcessCPU holds a process name and its CPU usage in percent
type PerProcessCPU struct {
	Name       string
	CPUPercent float64
}

// PerProcessDetails includes extended info for a process
type PerProcessDetails struct {
	PID        int32
	Name       string
	Username   string
	Cmdline    string
	CreateTime int64
}

// GetCPUUsagePercent returns total CPU usage (system-wide) as a percentage.
func GetCPUUsagePercent() float64 {
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

	for _, p := range procs {
		name, _ := p.Name()
		cpuPercent, err := p.CPUPercent()
		if err == nil {
			results = append(results, PerProcessCPU{
				Name:       name,
				CPUPercent: cpuPercent,
			})
		}
	}
	return results
}

// GetPerProcessDetails returns extended details per process
func GetPerProcessDetails() []PerProcessDetails {
	var details []PerProcessDetails
	procs, err := process.Processes()
	if err != nil {
		return details
	}

	for _, p := range procs {
		name, _ := p.Name()
		username, _ := p.Username()
		cmdline, _ := p.Cmdline()
		ctime, _ := p.CreateTime()

		details = append(details, PerProcessDetails{
			PID:        p.Pid,
			Name:       name,
			Username:   username,
			Cmdline:    cmdline,
			CreateTime: ctime,
		})
	}
	return details
}
