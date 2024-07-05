/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package client

import (
	"testing"

	"github.com/moresec-io/conduit/pkg/agent/config"
)

func TestSetTables(t *testing.T) {
	conf := &config.Config{}
	conf.Client.Policies = make([]config.Policy, 1)
	conf.Client.Policies[0].Dst = ":9092"
	conf.Client.Listen = "127.0.0.1:5052" // client
	t.Log(conf.Client.Policies[0].Dst)

	client, err := NewClient(conf)
	if err != nil {
		t.Error(err)
		return
	}
	client.initTables()
}

func TestUnSetTables(t *testing.T) {
	conf := &config.Config{}
	conf.Client.Policies = make([]config.Policy, 1)
	conf.Client.Policies[0].Dst = ":9092"
	conf.Client.Listen = "127.0.0.1:5052" // client
	t.Log(conf.Client.Policies[0].Dst)

	client, err := NewClient(conf)
	if err != nil {
		t.Error(err)
		return
	}
	client.finiTables("client fini tables")
}
