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

type NetFlowEntry struct {
	Interface string    `json:"interface"`
	Direction string    `json:"direction"` // inbound or outbound
	SrcIP     string    `json:"src_ip"`
	DstIP     string    `json:"dst_ip"`
	SrcPort   uint16    `json:"src_port"`
	DstPort   uint16    `json:"dst_port"`
	Protocol  string    `json:"protocol"`
	Packets   int       `json:"packets"`
	Bytes     int       `json:"bytes"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
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
			ipNet, ok := addr.(*net.IPNet)
			if ok && ipNet.IP.To4() != nil {
				ips[ipNet.IP.String()] = true
			}
		}
	}
	return ips
}

func makeFlowKey(iface string, src, dst string, sport, dport uint16, proto string, dir string) string {
	return strings.Join([]string{iface, src, dst, proto, dir, string(sport), string(dport)}, "|")
}

func updateFlow(entry *NetFlowEntry, length int) {
	entry.Packets++
	entry.Bytes += length
	entry.EndTime = time.Now()
}

func CaptureNetFlowFromAll(override []string) {
	ifaces := []pcap.Interface{}
	log.Printf("Monitoring interfaces:")
	for _, iface := range ifaces {
		log.Println("- " + iface.Name)
	}
	if len(override) == 0 {
		all, err := pcap.FindAllDevs()
		if err != nil {
			log.Printf("Error getting interfaces: %v", err)
			return
		}
		ifaces = all
	} else {
		for _, name := range override {
			iface, err := pcap.FindAllDevs()
			if err != nil {
				continue
			}
			for _, dev := range iface {
				if dev.Name == name {
					ifaces = append(ifaces, dev)
					break
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

		var (
			srcIP, dstIP string
			proto        string
			sport, dport uint16
		)

		// Safely handle IPv4 only
		switch ipLayer := networkLayer.(type) {
		case *layers.IPv4:
			srcIP = ipLayer.SrcIP.String()
			dstIP = ipLayer.DstIP.String()
			proto = ipLayer.Protocol.String()
		default:
			continue // skip non-IPv4 (like IPv6)
		}

		log.Printf("Packet captured on %s (src=%s, dst=%s)", name, srcIP, dstIP)

		dir := "inbound"
		if localIPs[srcIP] {
			dir = "outbound"
		}

		// Detect port and transport
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
				Interface: name,
				Direction: dir,
				SrcIP:     srcIP,
				DstIP:     dstIP,
				SrcPort:   sport,
				DstPort:   dport,
				Protocol:  proto,
				Packets:   1,
				Bytes:     len(packet.Data()),
				StartTime: now,
				EndTime:   now,
			}
		}
		netflowMu.Unlock()
	}
}

func GetNetFlowEntries() []NetFlowEntry {
	netflowMu.Lock()
	defer netflowMu.Unlock()

	result := []NetFlowEntry{}
	for _, v := range netflowBuffer {
		result = append(result, *v)
	}
	return result
}
