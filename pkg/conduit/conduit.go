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
	"github.com/moresec-io/conduit/pkg/conduit/syncer"
)

type Conduit struct {
	conf   *config.Config
	client *client.Client
	server *server.Server
}

func NewConduit() (*Conduit, error) {
	var (
		cli       *client.Client
		clienable bool
		srv       *server.Server
		srvenable bool
		syn       syncer.Syncer
		syncMode  int
		err       error
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

	if config.Conf.Client.Enable {
		clienable = true
		syncMode |= syncer.SyncModeDown
	}
	if config.Conf.Server.Enable {
		srvenable = true
		syncMode |= syncer.SyncModeUp
	}

	repo := repo.NewRepo()
	if config.Conf.Manager.Enable {
		syn, err = syncer.NewSyncer(config.Conf, repo, syncMode)
		if err != nil {
			log.Errorf("conduit new syncer err: %s", err)
			return nil, err
		}
	}

	if clienable {
		cli, err = client.NewClient(config.Conf, syn, repo)
		if err != nil {
			log.Errorf("conduit new client err: %s", err)
			return nil, err
		}
	}
	if srvenable {
		srv, err = server.NewServer(config.Conf, syn)
		if err != nil {
			log.Errorf("conduit new server err: %s", err)
			return nil, err
		}
	}
	return &Conduit{
		conf:   config.Conf,
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
