/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package conduit

import (
	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/conduit/client"
	"github.com/moresec-io/conduit/pkg/conduit/config"
	"github.com/moresec-io/conduit/pkg/conduit/repo"
	"github.com/moresec-io/conduit/pkg/conduit/server"
)

type Conduit struct {
	conf   *config.Config
	client *client.Client
	server *server.Server
}

func NewConduit() (*Conduit, error) {
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
                CONDUIT STARTS
==================================================`)

	conf := config.Conf

	repo := repo.NewRepo()

	if conf.Client.Enable {
		cli, err = client.NewClient(config.Conf, repo)
		if err != nil {
			log.Errorf("Conduit new client err: %s", err)
			return nil, err
		}
	}
	if conf.Server.Enable {
		srv, err = server.NewServer(conf)
		if err != nil {
			log.Errorf("Conduit new server err: %s", err)
			return nil, err
		}
	}
	return &Conduit{
		conf:   conf,
		client: cli,
		server: srv,
	}, nil
}

func (Conduit *Conduit) Run() {
	if Conduit.conf.Client.Enable {
		go Conduit.client.Work()
	}
	if Conduit.conf.Server.Enable {
		go Conduit.server.Work()
	}
}

func (Conduit *Conduit) Close() {
	defer func() {
		log.Infof(`
==================================================
                CONDUIT ENDS
==================================================`)
	}()
	if Conduit.conf.Client.Enable {
		Conduit.client.Close()
	}
	if Conduit.conf.Server.Enable {
		Conduit.server.Close()
	}
	config.RotateLog.Close()
}
