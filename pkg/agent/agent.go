package agent

import (
	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/agent/client"
	"github.com/moresec-io/conduit/pkg/agent/config"
	"github.com/moresec-io/conduit/pkg/agent/server"
)

type Agent struct {
	conf   *config.Config
	client *client.Client
	server *server.Server
}

func NewAgent() (*Agent, error) {
	var (
		cli *client.Client
		srv *server.Server
		err error
	)
	err = config.Init()
	if err != nil {
		log.Fatalf("init config err: %s", err)
		return nil, err
	}
	log.Infof(`
==================================================
                CONDUIT AGENT STARTS
==================================================`)

	conf := config.Conf

	if conf.Client.Enable {
		cli, err = client.NewClient(conf)
		if err != nil {
			log.Errorf("agent new client err: %s", err)
			return nil, err
		}
	}
	if conf.Server.Enable {
		srv, err = server.NewServer(conf)
		if err != nil {
			log.Errorf("agent new server err: %s", err)
			return nil, err
		}
	}
	return &Agent{
		conf:   conf,
		client: cli,
		server: srv,
	}, nil
}

func (agent *Agent) Run() {
	if agent.conf.Client.Enable {
		go agent.client.Work()
	}
	if agent.conf.Server.Enable {
		go agent.server.Work()
	}
}

func (agent *Agent) Close() {
	defer func() {
		log.Infof(`
==================================================
                CONDUIT AGENT ENDS
==================================================`)
	}()
	if agent.conf.Client.Enable {
		agent.client.Close()
	}
	if agent.conf.Server.Enable {
		agent.server.Close()
	}
	config.RotateLog.Close()
}
