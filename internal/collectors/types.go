package collectors

// OSInfo holds operating system information.
type OSInfo struct {
	Manufacturer      string
	Model             string
	Caption           string
	Version           string
	BuildNumber       string
	LogicalProcessors uint64
}

// ThermalZoneTemp holds thermal zone temperature information.
type ThermalZoneTemp struct {
	Instance    string
	TempCelsius float64
}

// EventLogStats holds event log statistics.
type EventLogStats struct {
	ErrorCount       uint64
	WarningCount     uint64
	InformationCount uint64
	OtherCount       uint64
}
