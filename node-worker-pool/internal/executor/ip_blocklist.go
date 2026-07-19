package executor

import (
	"net"
)

// IPBlocklist maintains lists of blocked IP ranges and hostnames
type IPBlocklist struct {
	blockedRanges    []*net.IPNet
	blockedHostnames map[string]bool
}

// NewIPBlocklist creates a blocklist with default private/internal ranges
func NewIPBlocklist() *IPBlocklist {
	bl := &IPBlocklist{
		blockedRanges:    make([]*net.IPNet, 0),
		blockedHostnames: make(map[string]bool),
	}

	// Private IPv4 ranges
	bl.addRange("10.0.0.0/8")
	bl.addRange("172.16.0.0/12")
	bl.addRange("192.168.0.0/16")
	
	// Loopback
	bl.addRange("127.0.0.0/8")
	bl.addRange("::1/128")
	
	// Link-local
	bl.addRange("169.254.0.0/16")
	bl.addRange("fe80::/10")
	
	// Multicast
	bl.addRange("224.0.0.0/4")
	
	// Cloud metadata IPs
	bl.addRange("169.254.169.254/32")
	bl.addRange("fd00:ec2::254/128")
	
	// Blocked hostnames
	bl.blockedHostnames["metadata.google.internal"] = true
	bl.blockedHostnames["metadata.azure.com"] = true
	bl.blockedHostnames["169.254.169.254"] = true
	bl.blockedHostnames["localhost"] = true

	return bl
}

func (bl *IPBlocklist) addRange(cidr string) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err == nil {
		bl.blockedRanges = append(bl.blockedRanges, ipNet)
	}
}

// IsBlocked checks if an IP address is in the blocklist
func (bl *IPBlocklist) IsBlocked(ip net.IP) bool {
	for _, ipNet := range bl.blockedRanges {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// IsHostnameBlocked checks if a hostname is blocked
func (bl *IPBlocklist) IsHostnameBlocked(hostname string) bool {
	return bl.blockedHostnames[hostname]
}
