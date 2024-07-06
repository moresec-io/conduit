package proto

const (
	RPCPullCluster = "pull_cluster"
)

type PullClusterRequest struct {
	MachineID string `json:"machine_id"`
}

type Agent ReportAgentRequest

type PullClusterResponse struct {
	Agents []Agent `json:"agents"`
}
