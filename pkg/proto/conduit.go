package proto

import "net"

const (
	RPCReportConduit  = "report_conduit"
	RPCOnlineConduit  = "online_conduit"
	RPCOfflineConduit = "offline_conduit"
)

type ReportConduitRequest struct {
	MachineID string      `json:"machine_id"`
	Network   string      `json:"network"`
	Listen    string      `json:"listen"`
	IPNets    []net.IPNet `json:"ipnets"`
}

type OnlineConduitRequest ReportConduitRequest

type OfflineConduitRequest struct {
	MachineID string `json:"machine_id"`
}
