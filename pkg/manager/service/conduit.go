package service

import (
	"context"
	"encoding/json"
	"net"

	"github.com/moresec-io/conduit/pkg/manager/cms"
	"github.com/moresec-io/conduit/pkg/proto"
	"github.com/singchia/geminio"
)

type ConduitType int

const (
	ConduitClient ConduitType = 1 << iota
	ConduitServer
)

type ServerConfig struct {
	Addr    string
	Network string
	Cert    *cms.Cert
	IPs     []net.IP
}

type Conduit interface {
	// caches
	SetClient()
	IsClient() bool
	GetServerConfig() *ServerConfig
	SetServer(*ServerConfig)
	SetServerIPs([]net.IP)
	IsServer() bool

	// events
	ServerOffline(machineID string) error
	ServerOnline(serverConduit *proto.Conduit) error
	ServerNetworksChanged(machineID string, ips []net.IP) error

	// meta
	MachineID() string

	// lifecycle
	Close() error
}

func NewConduit(end geminio.End) Conduit {
	return &conduit{
		end: end,
	}
}

type conduit struct {
	end          geminio.End
	typ          ConduitType
	serverConfig *ServerConfig
	machineID    string
}

// caches
func (conduit *conduit) SetClient() {
	conduit.typ |= ConduitClient
}

func (conduit *conduit) GetServerConfig() *ServerConfig {
	return conduit.serverConfig
}

func (conduit *conduit) SetServerIPs(ips []net.IP) {
	conduit.serverConfig.IPs = ips
}

func (conduit *conduit) SetServer(config *ServerConfig) {
	conduit.typ |= ConduitServer
	conduit.serverConfig = config
}

func (conduit *conduit) IsServer() bool {
	return (conduit.typ & ConduitServer) > 0
}

func (conduit *conduit) IsClient() bool {
	return (conduit.typ & ConduitClient) > 0
}

// events
func (conduit *conduit) ServerOffline(machineID string) error {
	request := &proto.SyncConduitOfflineRequest{
		MachineID: machineID,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req := conduit.end.NewRequest(data)
	rsp, err := conduit.end.Call(context.TODO(), proto.RPCSyncConduitOffline, req)
	if err != nil {
		return err
	}
	if rsp.Error() != nil {
		return rsp.Error()
	}
	return nil
}

func (conduit *conduit) ServerOnline(serverConduit *proto.Conduit) error {
	request := &proto.SyncConduitOnlineRequest{
		Conduit: serverConduit,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req := conduit.end.NewRequest(data)
	rsp, err := conduit.end.Call(context.TODO(), proto.RPCSyncConduitOnline, req)
	if err != nil {
		return err
	}
	if rsp.Error() != nil {
		return rsp.Error()
	}
	return nil
}

func (conduit *conduit) ServerNetworksChanged(machineID string, ips []net.IP) error {
	request := &proto.SyncConduitNetworksChangedRequest{
		MachineID: machineID,
		IPs:       ips,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req := conduit.end.NewRequest(data)
	rsp, err := conduit.end.Call(context.TODO(), proto.RPCSyncConduitNetworksChanged, req)
	if err != nil {
		return err
	}
	if rsp.Error() != nil {
		return rsp.Error()
	}
	return nil
}

// meta
func (conduit *conduit) MachineID() string {
	return conduit.machineID
}

func (conduit *conduit) Close() error {
	return conduit.end.Close()
}
