package utils

import (
	"net"
	"strings"
)

func CompareNets(old, new []net.IP) bool {
	if len(old) != len(new) {
		return false
	}
	for _, oldip := range old {
		found := false
		for _, newip := range new {
			if oldip.Equal(newip) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type IPs []net.IP

func (ips IPs) String() string {
	ipstrs := []string{}
	for _, ip := range ips {
		ipstrs = append(ipstrs, ip.String())
	}
	return strings.Join(ipstrs, ",")
}
