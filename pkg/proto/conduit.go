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

type TLS struct {
	Enable bool `json:"enable"`
	MTLS   bool `json:"mtls"`
}

type ReportServerRequest struct {
	MachineID string `json:"machine_id"`
	Network   string `json:"network"`
	Listen    string `json:"listen"`
}
