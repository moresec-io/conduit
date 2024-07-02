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
	conf.Client.Proxy.Transfers = make([]struct {
		Dst   string `yaml:"dst"`
		Proxy string `yaml:"proxy"`
		DstTo string `yaml:"dst_to"`
	}, 1)
	conf.Client.Proxy.Transfers[0].Dst = ":9092"
	conf.Client.Proxy.Listen = "127.0.0.1:5052" // client
	t.Log(conf.Client.Proxy.Transfers[0].Dst)

	client, err := NewClient(conf)
	if err != nil {
		t.Error(err)
		return
	}
	client.initTables()
}

func TestUnSetTables(t *testing.T) {
	conf := &config.Config{}
	conf.Client.Proxy.Transfers = make([]struct {
		Dst   string `yaml:"dst"`
		Proxy string `yaml:"proxy"`
		DstTo string `yaml:"dst_to"`
	}, 1)
	conf.Client.Proxy.Transfers[0].Dst = ":9092"
	conf.Client.Proxy.Listen = "127.0.0.1:5052" // client
	t.Log(conf.Client.Proxy.Transfers[0].Dst)

	client, err := NewClient(conf)
	if err != nil {
		t.Error(err)
		return
	}
	client.finiTables("client fini tables")
}
