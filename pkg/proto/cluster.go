package proto

const (
	RPCPullCluster = "pull_cluster"
)

type PullClusterRequest struct {
	MachineID string `json:"machine_id"`
}

type Conduit ReportConduitRequest

type PullClusterResponse struct {
	Conduits []Conduit `json:"conduits"`
}
