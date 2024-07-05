package proto

import "net"

const (
	RPCReportAgent = "report_agents"
)

type ReportAgentRequest struct {
	MachineID string      `json:"machine_id"`
	IPNets    []net.IPNet `json:"ipnets"`
}
