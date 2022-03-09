package main

import (
	"context"

	"github.com/jumboframes/conduit"
	"github.com/jumboframes/conduit/pkg/log"
	"github.com/jumboframes/conduit/pkg/sigaction"
	"github.com/jumboframes/conduit/proxy"
)

func main() {
	err := conduit.Init()
	if err != nil {
		log.Fatalf("main | init err: %s", err)
		return
	}

	log.Infof(`
==================================================
                 CONDUIT STARTS
==================================================`)

	if conduit.Conf.Server.Enable {
		server, err := proxy.NewServer(conduit.Conf)
		if err != nil {
			log.Errorf("main | new server err: %s", err)
			return
		}
		go server.Proxy()
	}

	if conduit.Conf.Client.Enable {
		client, err := proxy.NewClient(conduit.Conf)
		if err != nil {
			log.Errorf("main | new client err: %s", err)
			return
		}
		go client.Proxy()
	}

	sig := sigaction.NewSignal()
	sig.Wait(context.TODO())
}
