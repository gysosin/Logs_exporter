// internal/collectors/netflow_win.go
//go:build windows
// +build windows

package collectors

import (
	"encoding/binary"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/imgk/divert-go"
)

type NetFlowEntry struct {
	Interface  string    `json:"interface"`
	Direction  string    `json:"direction"` // inbound or outbound
	SrcIP      string    `json:"src_ip"`
	DstIP      string    `json:"dst_ip"`
	SrcPort    uint16    `json:"src_port"`
	DstPort    uint16    `json:"dst_port"`
	Protocol   string    `json:"protocol"`
	Packets    int       `json:"packets"`
	Bytes      int       `json:"bytes"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	SystemName string    `json:"system_name"` // ← new field
}

var (
	netflowMu     sync.Mutex
	netflowBuffer = make(map[string]*NetFlowEntry)
	localIPs      = getLocalIPs()
)

func getLocalIPs() map[string]bool {
	ips := make(map[string]bool)
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				ips[ipNet.IP.String()] = true
			}
		}
	}
	return ips
}

func makeFlowKey(iface, src, dst string, sport, dport uint16, proto, dir string) string {
	return strings.Join([]string{
		iface,
		src,
		dst,
		proto,
		dir,
		strconv.Itoa(int(sport)),
		strconv.Itoa(int(dport)),
	}, "|")
}

func updateFlow(entry *NetFlowEntry, length int) {
	entry.Packets++
	entry.Bytes += length
	entry.EndTime = time.Now()
}

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
		if flow != nil {
			// skip loopback
			if strings.HasPrefix(flow.SrcIP, "127.") && strings.HasPrefix(flow.DstIP, "127.") {
				handle.Send(packet[:n], addr)
				continue
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
				updateFlow(entry, int(n))
			} else {
				flow.Packets = 1
				flow.Bytes = int(n)
				netflowBuffer[key] = flow
			}
			netflowMu.Unlock()
		}

		// reinject
		handle.Send(packet[:n], addr)
	}
}

func parseIPv4Packet(data []byte) *NetFlowEntry {
	if len(data) < 20 {
		return nil
	}

	protocol := data[9]
	srcIP := net.IPv4(data[12], data[13], data[14], data[15]).String()
	dstIP := net.IPv4(data[16], data[17], data[18], data[19]).String()

	headerLen := int(data[0]&0x0F) * 4
	if len(data) < headerLen+4 {
		return nil
	}

	var sport, dport uint16
	var protoStr string

	switch protocol {
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

func GetNetFlowEntries() []NetFlowEntry {
	netflowMu.Lock()
	defer netflowMu.Unlock()

	result := make([]NetFlowEntry, 0, len(netflowBuffer))
	for _, v := range netflowBuffer {
		result = append(result, *v)
	}
	return result
}
