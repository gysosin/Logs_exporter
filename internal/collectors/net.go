package collectors

import (
    "github.com/shirou/gopsutil/v3/net"
)

// NetMetrics holds bytes sent/received
type NetMetrics struct {
    InterfaceName string
    BytesSent     float64
    BytesRecv     float64
}

// TCPUDPStats holds aggregated TCP/UDP counters
type TCPUDPStats struct {
    TCPConnectionsEstablished    uint64
    TCPConnectionsActive         uint64
    TCPConnectionsPassive        uint64
    TCPConnectionFailures        uint64
    UDPDatagramsReceivedErrors   uint64
    UDPDatagramsNoPort           uint64
}

// GetNetworkMetrics returns basic network counters (bytes sent/received) for each interface
func GetNetworkMetrics() []NetMetrics {
    var results []NetMetrics

    counters, err := net.IOCounters(true)
    if err != nil {
        return results
    }

    // We do not have an exact "bytes per second" unless we measure again and subtract.
    // For demonstration, we just return the total bytes counters as if they were "sent/sec".
    // If you need the actual rate, you'd measure periodically and compute the difference.
    for _, c := range counters {
        results = append(results, NetMetrics{
            InterfaceName: c.Name,
            BytesSent:     float64(c.BytesSent),
            BytesRecv:     float64(c.BytesRecv),
        })
    }
    return results
}

// GetTCPUDPStats attempts to read TCP/UDP counters using gopsutil net.ProtoCounters()
func GetTCPUDPStats() TCPUDPStats {
    var stats TCPUDPStats

    protoStats, err := net.ProtoCounters([]string{"tcp", "udp"})
    if err != nil {
        return stats
    }

    for _, ps := range protoStats {
        switch ps.Protocol {
        case "tcp":
            stats.TCPConnectionsEstablished = ps.Stats["CurrEstab"]
            stats.TCPConnectionsActive      = ps.Stats["ActiveOpens"]
            stats.TCPConnectionsPassive     = ps.Stats["PassiveOpens"]
            stats.TCPConnectionFailures     = ps.Stats["AttemptFails"]
        case "udp":
            stats.UDPDatagramsReceivedErrors = ps.Stats["InErrors"]
            stats.UDPDatagramsNoPort         = ps.Stats["NoPorts"]
        }
    }
    return stats
}
