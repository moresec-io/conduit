/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package main

import (
	"context"

	"github.com/jumboframes/armorigo/sigaction"
	"github.com/moresec-io/conduit"
	"github.com/moresec-io/conduit/pkg/log"
	"github.com/moresec-io/conduit/proxy"
	//_ "net/http/pprof"
)

func main() {
	err := conduit.Init()
	if err != nil {
		log.Fatalf("main | init err: %s", err)
		return
	}
	defer conduit.RotateLog.Close()

	//go http.ListenAndServe("0.0.0.0:6060", nil)

	err = conduit.SetRLimit(1024 * 1024)
	if err != nil {
		log.Fatalf("main | set rlimit err: %s", err)
		return
	}

	log.Infof(`
==================================================
                 CONDUIT STARTS
==================================================`)

	var server *proxy.Server
	var client *proxy.Client

	if conduit.Conf.Server.Enable {
		server, err = proxy.NewServer(conduit.Conf)
		if err != nil {
			log.Errorf("main | new server err: %s", err)
			return
		}
		go server.Work()
	}

	if conduit.Conf.Client.Enable {
		client, err = proxy.NewClient(conduit.Conf)
		if err != nil {
			log.Errorf("main | new client err: %s", err)
			return
		}
		go client.Work()
	}

	sig := sigaction.NewSignal()
	sig.Wait(context.TODO())

	if conduit.Conf.Server.Enable {
		server.Close()
	}

	if conduit.Conf.Client.Enable {
		client.Close()
	}

	log.Infof(`
==================================================
                 CONDUIT ENDS
==================================================`)
}
