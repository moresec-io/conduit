package proto

import "net"

const (
	RPCPullCluster = "pull_cluster"
)

type PullClusterRequest struct {
	MachineID string `json:"machine_id"`
}

type Nets struct {
	MachineID string      `json:"machine_id"`
	IPNets    []net.IPNet `json:"ipnets"`
}

type PullClusterResponse struct {
	Cluster []Nets `json:"conduits"`
}
