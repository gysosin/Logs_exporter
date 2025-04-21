// internal/collectors/netflow.go
//go:build !windows
// +build !windows

package collectors

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

var (
	// when false, skip host<->host (private) flows
	IncludeInternalFlows = false

	privateCIDRs []*net.IPNet
)

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

func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, net := range privateCIDRs {
		if net.Contains(ip) {
			return true
		}
	}
	return false
}

// NetFlowEntry holds a single aggregated flow
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
	return strings.Join([]string{iface, src, dst, proto, dir, string(sport), string(dport)}, "|")
}

func updateFlow(entry *NetFlowEntry, length int) {
	entry.Packets++
	entry.Bytes += length
	entry.EndTime = time.Now()
}

func CaptureNetFlowFromAll(override []string) {
	var ifaces []pcap.Interface
	if len(override) == 0 {
		all, err := pcap.FindAllDevs()
		if err != nil {
			log.Printf("Error getting interfaces: %v", err)
			return
		}
		ifaces = all
	} else {
		devs, _ := pcap.FindAllDevs()
		for _, name := range override {
			for _, dev := range devs {
				if dev.Name == name {
					ifaces = append(ifaces, dev)
				}
			}
		}
	}
	for _, iface := range ifaces {
		go captureFromInterface(iface.Name)
	}
}

func captureFromInterface(name string) {
	handle, err := pcap.OpenLive(name, 1600, true, pcap.BlockForever)
	if err != nil {
		log.Printf("Error opening interface %s: %v", name, err)
		return
	}
	defer handle.Close()

	src := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range src.Packets() {
		networkLayer := packet.NetworkLayer()
		if networkLayer == nil {
			continue
		}

		var srcIP, dstIP, proto string
		var sport, dport uint16

		switch ipLayer := networkLayer.(type) {
		case *layers.IPv4:
			srcIP = ipLayer.SrcIP.String()
			dstIP = ipLayer.DstIP.String()
			proto = ipLayer.Protocol.String()
		default:
			continue
		}

		// direction
		dir := "inbound"
		if localIPs[srcIP] {
			dir = "outbound"
		}

		// FILTER: drop purely host<->host private traffic
		if !IncludeInternalFlows {
			if localIPs[srcIP] && isPrivateIP(dstIP) {
				continue
			}
			if localIPs[dstIP] && isPrivateIP(srcIP) {
				continue
			}
		}

		// transport ports
		if t := packet.TransportLayer(); t != nil {
			switch layer := t.(type) {
			case *layers.TCP:
				sport, dport = uint16(layer.SrcPort), uint16(layer.DstPort)
			case *layers.UDP:
				sport, dport = uint16(layer.SrcPort), uint16(layer.DstPort)
			}
		}

		key := makeFlowKey(name, srcIP, dstIP, sport, dport, proto, dir)
		now := time.Now()

		netflowMu.Lock()
		if entry, ok := netflowBuffer[key]; ok {
			updateFlow(entry, len(packet.Data()))
		} else {
			netflowBuffer[key] = &NetFlowEntry{
				Interface:  name,
				Direction:  dir,
				SrcIP:      srcIP,
				DstIP:      dstIP,
				SrcPort:    sport,
				DstPort:    dport,
				Protocol:   proto,
				Packets:    1,
				Bytes:      len(packet.Data()),
				StartTime:  now,
				EndTime:    now,
				SystemName: "",
			}
		}
		netflowMu.Unlock()
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
