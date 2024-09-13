package proto

import "net"

const (
	RPCReportConduit  = "report_conduit"
	RPCOnlineConduit  = "online_conduit"
	RPCOfflineConduit = "offline_conduit"
)

type ReportConduitRequest struct {
	MachineID string      `json:"machine_id"`
	IPNets    []net.IPNet `json:"ipnets"`
}

type OnlineConduitRequest ReportConduitRequest

type OfflineConduitRequest struct {
	MachineID string `json:"machine_id"`
}

type TLS struct {
	Enable bool   `json:"enable"`
	MTLS   bool   `json:"mtls"`
	CA     []byte `json:"ca"`
	Cert   []byte `json:"cert"`
	Key    []byte `json:"key"`
}

type ReportServerRequest struct {
	MachineID string `json:"machine_id"`
	Network   string `json:"network"`
	Addr      string `json:"addr"`
}

type ReportServerResponse struct {
	TLS *TLS
}
