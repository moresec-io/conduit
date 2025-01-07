package proto

import "net"

const (
	// client or server report to manager
	RPCReportServer   = "report_server"
	RPCReportClient   = "report_cliet"
	RPCReportNetworks = "report_networks"

	// manager report to server
	RPCSyncConduitOnline          = "sync_conduit_online"
	RPCSyncConduitOffline         = "sync_conduit_offline"
	RPCSyncConduitNetworksChanged = "sync_conduit_networks_changed"
)

// manager sync to clients
type SyncConduitOnlineRequest struct {
	Conduit *Conduit
}

// manager sync to clients
type SyncConduitOfflineRequest struct {
	MachineID string `json:"machine_id"`
}

// manager sync to clients
type SyncConduitNetworksChangedRequest struct {
	MachineID string   `json:"machine_id"`
	IPs       []net.IP `json:"ips"`
}

type TLS struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}

// server report to manager
type ReportServerRequest struct {
	MachineID string   `json:"machine_id"`
	Network   string   `json:"network"`
	Addr      string   `json:"addr"`
	IPs       []net.IP `json:"ips"`
}

type ReportServerResponse struct {
	TLS *TLS
}

// server report to manager
type ReportClientRequest struct {
	MachineID string `json:"machine_id"`
}

type ReportClientResponse struct {
	TLS *TLS
}

// server report to manager
type ReportNetworksRequest struct {
	MachineID string   `json:"machine_id"`
	IPs       []net.IP `json:"ips"`
}
