package proto

import "net"

const (
	RPCPullCluster = "pull_cluster"
)

type PullClusterRequest struct {
	MachineID string `json:"machine_id"`
}

type Conduit struct {
	MachineID string `json:"machine_id"`
	Network   string
	Addr      string
	IPs       []net.IP `json:"ips"`
	// IPNets []net.IPNet `json:"ipnets"` // unsupported yet
}

type PullClusterResponse struct {
	Cluster []Conduit `json:"conduits"`
}
