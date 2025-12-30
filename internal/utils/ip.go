package utils

import (
	"net"
)

// IsAllowedIP checking if the IP address enters the allowed CIDR subnetwork
func IsAllowedIP(ip string, allowedCIDRs []string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	for _, cidr := range allowedCIDRs {
		_, netblock, err := net.ParseCIDR(cidr)
		if err != nil {
			// Skip invalid CIDR
			continue
		}
		if netblock.Contains(parsed) {
			return true
		}
	}
	return false
}
