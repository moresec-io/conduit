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
	Host string
	Port int
	Cert *cms.Cert
	IPs  []net.IP
}

type Conduit interface {
	SetClient()
	SetServer(*ServerConfig)
	IsClient() bool
	IsServer() bool
	ServerOffline(machineID string) error
	MachineID() string
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

func (conduit *conduit) SetClient() {
	conduit.typ |= ConduitClient
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

func (conduit *conduit) ServerOffline(machineID string) error {
	request := &proto.OfflineConduitRequest{
		MachineID: machineID,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req := conduit.end.NewRequest(data)
	rsp, err := conduit.end.Call(context.TODO(), proto.RPCOfflineConduit, req)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	if rsp.Error() != nil {
		return rsp.Error()
	}
	return nil
}

func (conduit *conduit) MachineID() string {
	return conduit.machineID
}
