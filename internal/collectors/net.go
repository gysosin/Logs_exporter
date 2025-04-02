package collectors

import (
	"github.com/shirou/gopsutil/v3/net"
)

// NetMetrics holds bytes sent/received.
type NetMetrics struct {
	InterfaceName string
	BytesSent     float64
	BytesRecv     float64
}

// TCPUDPStats holds aggregated TCP/UDP counters.
type TCPUDPStats struct {
	TCPConnectionsEstablished  uint64
	TCPConnectionsActive       uint64
	TCPConnectionsPassive      uint64
	TCPConnectionFailures      uint64
	UDPDatagramsReceivedErrors uint64
	UDPDatagramsNoPort         uint64
}

// GetNetworkMetrics returns basic network counters for each interface.
func GetNetworkMetrics() []NetMetrics {
	var results []NetMetrics

	counters, err := net.IOCounters(true)
	if err != nil {
		return results
	}

	for _, c := range counters {
		results = append(results, NetMetrics{
			InterfaceName: c.Name,
			BytesSent:     float64(c.BytesSent),
			BytesRecv:     float64(c.BytesRecv),
		})
	}
	return results
}

// GetTCPUDPStats returns TCP/UDP statistics with explicit type casts.
func GetTCPUDPStats() TCPUDPStats {
	var stats TCPUDPStats

	protoStats, err := net.ProtoCounters([]string{"tcp", "udp"})
	if err != nil {
		return stats
	}

	for _, ps := range protoStats {
		switch ps.Protocol {
		case "tcp":
			stats.TCPConnectionsEstablished = uint64(ps.Stats["CurrEstab"])
			stats.TCPConnectionsActive = uint64(ps.Stats["ActiveOpens"])
			stats.TCPConnectionsPassive = uint64(ps.Stats["PassiveOpens"])
			stats.TCPConnectionFailures = uint64(ps.Stats["AttemptFails"])
		case "udp":
			stats.UDPDatagramsReceivedErrors = uint64(ps.Stats["InErrors"])
			stats.UDPDatagramsNoPort = uint64(ps.Stats["NoPorts"])
		}
	}
	return stats
}
