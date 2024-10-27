package service

import (
	"github.com/moresec-io/conduit/pkg/manager/cms"
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
}

type Conduit interface {
	SetClient()
	SetServer(*ServerConfig)
	IsServer() bool
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
