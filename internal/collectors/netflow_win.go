// internal/collectors/netflow_win.go
//go:build windows
// +build windows

package collectors

import (
	"encoding/binary"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/imgk/divert-go"
)

// When false, purely host↔host (private) traffic is skipped
var IncludeInternalFlows = false

// privateCIDRs holds the RFC1918 (and loopback) networks
var privateCIDRs []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	} {
		_, network, _ := net.ParseCIDR(cidr)
		privateCIDRs = append(privateCIDRs, network)
	}
}

// isPrivateIP returns true if ipStr falls in a private or loopback range
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, network := range privateCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// getLocalIPs enumerates all IPv4 addresses assigned to local interfaces
func getLocalIPs() map[string]bool {
	out := make(map[string]bool)
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				out[ipNet.IP.String()] = true
			}
		}
	}
	return out
}

// makeFlowKey produces a stable key for aggregation
func makeFlowKey(iface, src, dst string, sport, dport uint16, proto, dir string) string {
	// we use strconv.Itoa on ports to ensure uniqueness
	parts := []string{iface, src, dst, proto, dir, strconv.Itoa(int(sport)), strconv.Itoa(int(dport))}
	return strings.Join(parts, "|")
}

type NetFlowEntry struct {
	Interface  string    `json:"interface"`
	Direction  string    `json:"direction"`
	SrcIP      string    `json:"src_ip"`
	DstIP      string    `json:"dst_ip"`
	SrcPort    uint16    `json:"src_port"`
	DstPort    uint16    `json:"dst_port"`
	Protocol   string    `json:"protocol"`
	Packets    int       `json:"packets"`
	Bytes      int       `json:"bytes"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	SystemName string    `json:"system_name"`
}

var (
	netflowMu     sync.Mutex
	netflowBuffer = make(map[string]*NetFlowEntry)
	localIPs      = getLocalIPs()
)

// CaptureNetFlowFromAll starts WinDivert capture in a goroutine.
func CaptureNetFlowFromAll(_ []string) {
	go captureWithWinDivert()
}

func captureWithWinDivert() {
	filter := "ip and (tcp or udp)"
	handle, err := divert.Open(filter, divert.LayerNetwork, 0, 0)
	if err != nil {
		log.Printf("WinDivert open failed: %v", err)
		return
	}
	defer handle.Close()

	packet := make([]byte, 65535)
	addr := new(divert.Address)

	for {
		n, err := handle.Recv(packet, addr)
		if err != nil {
			log.Printf("WinDivert recv error: %v", err)
			continue
		}

		flow := parseIPv4Packet(packet[:n])
		if flow == nil {
			handle.Send(packet[:n], addr) // reinject unmodified
			continue
		}

		// always skip pure loopback
		if strings.HasPrefix(flow.SrcIP, "127.") || strings.HasPrefix(flow.DstIP, "127.") {
			handle.Send(packet[:n], addr)
			continue
		}

		// skip host↔host private traffic if disabled
		if !IncludeInternalFlows {
			if localIPs[flow.SrcIP] && isPrivateIP(flow.DstIP) {
				handle.Send(packet[:n], addr)
				continue
			}
			if localIPs[flow.DstIP] && isPrivateIP(flow.SrcIP) {
				handle.Send(packet[:n], addr)
				continue
			}
		}

		flow.Interface = "WinDivert"
		flow.Direction = "inbound"
		if localIPs[flow.SrcIP] {
			flow.Direction = "outbound"
		}

		now := time.Now()
		flow.StartTime = now
		flow.EndTime = now
		flow.SystemName = "" // will be set by pushMetrics

		key := makeFlowKey(flow.Interface, flow.SrcIP, flow.DstIP, flow.SrcPort, flow.DstPort, flow.Protocol, flow.Direction)

		netflowMu.Lock()
		if entry, ok := netflowBuffer[key]; ok {
			entry.Packets++
			entry.Bytes += int(n)
			entry.EndTime = now
		} else {
			flow.Packets = 1
			flow.Bytes = int(n)
			netflowBuffer[key] = flow
		}
		netflowMu.Unlock()

		handle.Send(packet[:n], addr) // reinject
	}
}

func parseIPv4Packet(data []byte) *NetFlowEntry {
	if len(data) < 20 {
		return nil
	}
	protoNum := data[9]
	srcIP := net.IPv4(data[12], data[13], data[14], data[15]).String()
	dstIP := net.IPv4(data[16], data[17], data[18], data[19]).String()

	headerLen := int(data[0]&0x0F) * 4
	if len(data) < headerLen+4 {
		return nil
	}

	var sport, dport uint16
	var protoStr string
	switch protoNum {
	case 6: // TCP
		sport = binary.BigEndian.Uint16(data[headerLen : headerLen+2])
		dport = binary.BigEndian.Uint16(data[headerLen+2 : headerLen+4])
		protoStr = "tcp"
	case 17: // UDP
		sport = binary.BigEndian.Uint16(data[headerLen : headerLen+2])
		dport = binary.BigEndian.Uint16(data[headerLen+2 : headerLen+4])
		protoStr = "udp"
	default:
		return nil
	}

	return &NetFlowEntry{
		SrcIP:    srcIP,
		DstIP:    dstIP,
		SrcPort:  sport,
		DstPort:  dport,
		Protocol: protoStr,
	}
}

// GetNetFlowEntries returns a slice of all active flow entries.
func GetNetFlowEntries() []NetFlowEntry {
	netflowMu.Lock()
	defer netflowMu.Unlock()

	out := make([]NetFlowEntry, 0, len(netflowBuffer))
	for _, v := range netflowBuffer {
		out = append(out, *v)
	}
	// optional: sort by timestamp so results are deterministic
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartTime.Before(out[j].StartTime)
	})
	return out
}
