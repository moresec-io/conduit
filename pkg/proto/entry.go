package proto

import "net"

type Networks struct {
	IPs      []net.IP // standalone ip
	Networks []*Network
}

type Network struct {
	Gateway net.IPNet
	IPs     []net.IP
}

type Entry struct {
	MachineID string
	Networks  Networks
}
