package service

import "github.com/singchia/geminio"

type ConduitType int

const (
	ConduitClient ConduitType = 1 << iota
	ConduitServer
)

type Conduit interface {
	SetClient()
	SetServer()
}

func NewConduit(end geminio.End) Conduit {
	return &conduit{
		end: end,
	}
}

type conduit struct {
	end geminio.End
	typ ConduitType
}

func (conduit *conduit) SetClient() {
	conduit.typ |= ConduitClient
}

func (conduit *conduit) SetServer() {
	conduit.typ |= ConduitServer
}
