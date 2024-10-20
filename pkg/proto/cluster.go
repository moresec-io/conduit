package proto

import "net"

const (
	RPCPullCluster = "pull_cluster"
)

type PullClusterRequest struct {
	MachineID string `json:"machine_id"`
}

type Conduit struct {
	MachineID string      `json:"machine_id"`
	IPNets    []net.IPNet `json:"ipnets"`
}

type PullClusterResponse struct {
	Conduits []Conduit `json:"conduits"`
}
