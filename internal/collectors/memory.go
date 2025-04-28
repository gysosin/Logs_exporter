package collectors

import (
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// MemStats holds total, used, free
type MemStats struct {
	Total uint64
	Used  uint64
	Free  uint64
}

// PerProcessMem holds process name and working set
type PerProcessMem struct {
	Name        string
	MemoryBytes uint64
}

// GetMemoryMetrics returns total, used, free system memory in bytes.
func GetMemoryMetrics() MemStats {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return MemStats{}
	}

	return MemStats{
		Total: vm.Total,
		Used:  vm.Used,
		Free:  vm.Free,
	}
}

// GetPerProcessMemory returns working set sizes for each process
func GetPerProcessMemory() []PerProcessMem {
	var results []PerProcessMem

	procs, err := process.Processes()
	if err != nil {
		return results
	}

	for _, p := range procs {
		name, _ := p.Name()
		meminfo, err := p.MemoryInfo()
		if err == nil && meminfo != nil {
			results = append(results, PerProcessMem{
				Name:        name,
				MemoryBytes: meminfo.RSS, // or .VMS, etc.
			})
		}
	}
	return results
}
