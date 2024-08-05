package server

import (
	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/manager/config"
	"github.com/moresec-io/conduit/pkg/manager/repo"
	"github.com/moresec-io/conduit/pkg/network"
	"github.com/soheilhy/cmux"
)

type Server struct{}

func NewServer(conf *config.Config, repo repo.Repo) (*Server, error) {
	listen := &conf.ControlPlane.Listen
	ln, err := network.Listen(listen)
	if err != nil {
		log.Errorf("server listen err: %s", err)
		return nil, err
	}
	server := &Server{}

	// http and geminio server
	cm := cmux.New(ln)
	// the first byte is geminio Version, the second byte is geminio ConnPacket
	// TODO we should have a magic number here
	_ = cm.Match(cmux.PrefixMatcher(string([]byte{0x01, 0x01})))
	_ = cm.Match(cmux.Any())
	// TODO
	return server, nil
}
