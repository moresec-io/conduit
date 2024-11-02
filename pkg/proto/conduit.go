package proto

import "net"

const (
	RPCReportServer   = "report_server"
	RPCReportClient   = "report_cliet"
	RPCReportNetworks = "report_networks"
	RPCOnlineConduit  = "online_conduit"
	RPCOfflineConduit = "offline_conduit"
)

type ReportNetworksRequest struct {
	MachineID string   `json:"machine_id"`
	IPs       []net.IP `json:"ips"`
}

type OnlineConduitRequest struct {
	Conduit *Conduit
}

type OfflineConduitRequest struct {
	MachineID string `json:"machine_id"`
}

type TLS struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}

type ReportServerRequest struct {
	MachineID string `json:"machine_id"`
	Network   string `json:"network"`
	Addr      string `json:"addr"`
}

type ReportServerResponse struct {
	TLS *TLS
}

type ReportClientRequest struct {
	MachineID string `json:"machine_id"`
}

type ReportClientResponse struct {
	TLS *TLS
}
