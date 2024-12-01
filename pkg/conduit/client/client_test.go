/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package client

import (
	"testing"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/conduit/config"
	"github.com/moresec-io/conduit/pkg/conduit/repo"
)

func TestSetTables(t *testing.T) {
	conf := &config.Config{}
	conf.Client.ForwardTable = make([]config.ForwardElem, 1)
	conf.Client.ForwardTable[0].Dst = ":9092"
	conf.Client.Listen = "127.0.0.1:5052" // client
	t.Log(conf.Client.ForwardTable[0].Dst)

	client, err := NewClient(conf, nil, repo.NewRepo())
	if err != nil {
		t.Error(err)
		return
	}
	client.initTables()
}

func TestUnSetTables(t *testing.T) {
	conf := &config.Config{}
	conf.Client.ForwardTable = make([]config.ForwardElem, 1)
	conf.Client.ForwardTable[0].Dst = ":9092"
	conf.Client.Listen = "127.0.0.1:5052" // client
	t.Log(conf.Client.ForwardTable[0].Dst)

	client, err := NewClient(conf, nil, repo.NewRepo())
	if err != nil {
		t.Error(err)
		return
	}
	client.finiTables(log.LevelError, "client fini tables")
}
