package collectors

import (
	"fmt"
	"strings"
)

func GenerateMetrics() string {
	var sb strings.Builder

	// CPU (system-wide)
	cpuUsage := GetCPUUsagePercent()
	sb.WriteString("# HELP logs_exporter_cpu_usage_percent CPU usage in percent (system-wide).\n")
	sb.WriteString("# TYPE logs_exporter_cpu_usage_percent gauge\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_cpu_usage_percent %.2f\n\n", cpuUsage))

	// CPU (per-process)
	perProcCPU := GetPerProcessCPU()
	sb.WriteString("# HELP logs_exporter_process_cpu_percent CPU usage per process.\n")
	sb.WriteString("# TYPE logs_exporter_process_cpu_percent gauge\n")
	for _, p := range perProcCPU {
		safeName := strings.ReplaceAll(p.Name, `"`, "")
		sb.WriteString(fmt.Sprintf("logs_exporter_process_cpu_percent{process=\"%s\"} %.2f\n", safeName, p.CPUPercent))
	}
	sb.WriteString("\n")

	// Memory (system-wide)
	mem := GetMemoryMetrics()
	sb.WriteString("# HELP logs_exporter_memory_bytes System memory usage in bytes (total/used/free).\n")
	sb.WriteString("# TYPE logs_exporter_memory_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_memory_bytes{type=\"total\"} %d\n", mem.Total))
	sb.WriteString(fmt.Sprintf("logs_exporter_memory_bytes{type=\"used\"} %d\n", mem.Used))
	sb.WriteString(fmt.Sprintf("logs_exporter_memory_bytes{type=\"free\"} %d\n\n", mem.Free))

	// Memory (per-process)
	perProcMem := GetPerProcessMemory()
	sb.WriteString("# HELP logs_exporter_process_memory_bytes Process working set size in bytes.\n")
	sb.WriteString("# TYPE logs_exporter_process_memory_bytes gauge\n")
	for _, pm := range perProcMem {
		safeName := strings.ReplaceAll(pm.Name, `"`, "")
		sb.WriteString(fmt.Sprintf("logs_exporter_process_memory_bytes{process=\"%s\"} %d\n", safeName, pm.MemoryBytes))
	}
	sb.WriteString("\n")

	// Disk metrics
	diskMetrics := GetDiskMetrics()
	sb.WriteString("# HELP logs_exporter_disk_bytes Disk metrics in bytes per drive (total/used/free).\n")
	sb.WriteString("# TYPE logs_exporter_disk_bytes gauge\n")
	for _, d := range diskMetrics {
		safeDev := strings.ReplaceAll(d.Device, `"`, "")
		sb.WriteString(fmt.Sprintf("logs_exporter_disk_bytes{device=\"%s\",type=\"total\"} %d\n", safeDev, d.Total))
		sb.WriteString(fmt.Sprintf("logs_exporter_disk_bytes{device=\"%s\",type=\"used\"} %d\n", safeDev, d.Used))
		sb.WriteString(fmt.Sprintf("logs_exporter_disk_bytes{device=\"%s\",type=\"free\"} %d\n", safeDev, d.Free))
	}
	sb.WriteString("\n")

	// Volume metrics
	vols := GetVolumeMetrics()
	sb.WriteString("# HELP logs_exporter_volume_bytes Volume metrics in bytes (Size/Free).\n")
	sb.WriteString("# TYPE logs_exporter_volume_bytes gauge\n")
	for _, v := range vols {
		safeLabel := strings.ReplaceAll(v.FileSystemLabel, `"`, "")
		safeLetter := strings.ReplaceAll(v.DriveLetter, `"`, "")
		sb.WriteString(fmt.Sprintf("logs_exporter_volume_bytes{driveLetter=\"%s\", label=\"%s\", type=\"total\"} %d\n", safeLetter, safeLabel, v.SizeBytes))
		sb.WriteString(fmt.Sprintf("logs_exporter_volume_bytes{driveLetter=\"%s\", label=\"%s\", type=\"free\"} %d\n", safeLetter, safeLabel, v.FreeBytes))
	}
	sb.WriteString("\n")

	// Uptime
	uptime := GetUptime()
	sb.WriteString("# HELP logs_exporter_uptime_seconds System uptime in seconds.\n")
	sb.WriteString("# TYPE logs_exporter_uptime_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_uptime_seconds %d\n\n", uptime))

	// Network
	netMetrics := GetNetworkMetrics()
	sb.WriteString("# HELP logs_exporter_network_bytes_per_sec Network bytes per second per interface (sent/received).\n")
	sb.WriteString("# TYPE logs_exporter_network_bytes_per_sec gauge\n")
	for _, nm := range netMetrics {
		safeIface := strings.ReplaceAll(nm.InterfaceName, `"`, "")
		sb.WriteString(fmt.Sprintf("logs_exporter_network_bytes_per_sec{interface=\"%s\",type=\"sent\"} %.2f\n", safeIface, nm.BytesSent))
		sb.WriteString(fmt.Sprintf("logs_exporter_network_bytes_per_sec{interface=\"%s\",type=\"received\"} %.2f\n", safeIface, nm.BytesRecv))
	}
	sb.WriteString("\n")

	// TCP/UDP stats
	tcpudp := GetTCPUDPStats()
	sb.WriteString("# HELP logs_exporter_tcp_connections_established Number of currently established TCP connections.\n")
	sb.WriteString("# TYPE logs_exporter_tcp_connections_established gauge\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_tcp_connections_established %d\n\n", tcpudp.TCPConnectionsEstablished))

	sb.WriteString("# HELP logs_exporter_tcp_connections_active Number of active TCP openings.\n")
	sb.WriteString("# TYPE logs_exporter_tcp_connections_active gauge\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_tcp_connections_active %d\n\n", tcpudp.TCPConnectionsActive))

	sb.WriteString("# HELP logs_exporter_tcp_connections_passive Number of passive TCP openings.\n")
	sb.WriteString("# TYPE logs_exporter_tcp_connections_passive gauge\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_tcp_connections_passive %d\n\n", tcpudp.TCPConnectionsPassive))

	sb.WriteString("# HELP logs_exporter_tcp_connection_failures Number of failed TCP connections.\n")
	sb.WriteString("# TYPE logs_exporter_tcp_connection_failures counter\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_tcp_connection_failures %d\n\n", tcpudp.TCPConnectionFailures))

	sb.WriteString("# HELP logs_exporter_udp_datagrams_received_errors Number of UDP datagrams received with errors.\n")
	sb.WriteString("# TYPE logs_exporter_udp_datagrams_received_errors counter\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_udp_datagrams_received_errors %d\n\n", tcpudp.UDPDatagramsReceivedErrors))

	sb.WriteString("# HELP logs_exporter_udp_datagrams_noport Number of UDP datagrams received for nonexistent port.\n")
	sb.WriteString("# TYPE logs_exporter_udp_datagrams_noport counter\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_udp_datagrams_noport %d\n\n", tcpudp.UDPDatagramsNoPort))

	// Windows-only (safely handle nils/empties)
	for _, pf := range GetPageFileUsage() {
		safeFile := strings.ReplaceAll(pf.PageFile, `"`, "")
		sb.WriteString(fmt.Sprintf("logs_exporter_pagefile_usage_percent{pagefile=\"%s\"} %.2f\n", safeFile, pf.UsagePct))
	}
	sb.WriteString("\n")

	for _, svc := range GetServices() {
		safeName := strings.ReplaceAll(svc.Name, `"`, "")
		safeDisplay := strings.ReplaceAll(svc.Display, `"`, "")
		sb.WriteString(fmt.Sprintf("logs_exporter_service_state{name=\"%s\",display=\"%s\"} %d\n", safeName, safeDisplay, svc.StateValue))
		sb.WriteString(fmt.Sprintf("logs_exporter_service_start_mode{name=\"%s\",display=\"%s\"} %d\n", safeName, safeDisplay, svc.StartValue))
	}
	sb.WriteString("\n")

	procCount := GetProcessCount()
	sb.WriteString("# HELP logs_exporter_process_count Total number of processes on the system.\n")
	sb.WriteString("# TYPE logs_exporter_process_count gauge\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_process_count %d\n\n", procCount))

	osInfo := GetOSInfo()
	sb.WriteString("# HELP logs_exporter_system_info Static system information (labels only).\n")
	sb.WriteString("# TYPE logs_exporter_system_info gauge\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_system_info{manufacturer=\"%s\",model=\"%s\",caption=\"%s\",version=\"%s\",build=\"%s\"} 1\n\n",
		escapeQuotes(osInfo.Manufacturer),
		escapeQuotes(osInfo.Model),
		escapeQuotes(osInfo.Caption),
		escapeQuotes(osInfo.Version),
		escapeQuotes(osInfo.BuildNumber),
	))

	sb.WriteString("# HELP logs_exporter_system_logical_processors Number of logical processors in the system.\n")
	sb.WriteString("# TYPE logs_exporter_system_logical_processors gauge\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_system_logical_processors %d\n\n", osInfo.LogicalProcessors))

	tzones := GetThermalZoneTemps()
	sb.WriteString("# HELP logs_exporter_thermalzone_celsius Thermal zone temperature in Celsius.\n")
	sb.WriteString("# TYPE logs_exporter_thermalzone_celsius gauge\n")
	for _, tz := range tzones {
		safeInst := strings.ReplaceAll(tz.Instance, `"`, "")
		sb.WriteString(fmt.Sprintf("logs_exporter_thermalzone_celsius{instance=\"%s\"} %.2f\n", safeInst, tz.TempCelsius))
	}
	sb.WriteString("\n")

	evStats := GetEventLogStats()
	sb.WriteString("# HELP logs_exporter_event_log_count Number of events in the System log by type in the last hour.\n")
	sb.WriteString("# TYPE logs_exporter_event_log_count gauge\n")
	sb.WriteString(fmt.Sprintf("logs_exporter_event_log_count{level=\"Error\"} %d\n", evStats.ErrorCount))
	sb.WriteString(fmt.Sprintf("logs_exporter_event_log_count{level=\"Warning\"} %d\n", evStats.WarningCount))
	sb.WriteString(fmt.Sprintf("logs_exporter_event_log_count{level=\"Information\"} %d\n", evStats.InformationCount))
	sb.WriteString(fmt.Sprintf("logs_exporter_event_log_count{level=\"Other\"} %d\n", evStats.OtherCount))
	sb.WriteString("\n")

	return sb.String()
}

func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
