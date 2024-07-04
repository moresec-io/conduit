package proto

import "net"

type Entry struct {
	MachineID string
	IPNets    []net.IPNet
}
