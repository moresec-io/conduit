/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package client

import (
	"strings"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/network"
	"github.com/moresec-io/conduit/pkg/utils"
)

func (client *Client) setProc() error {
	err := client.initProc()
	if err != nil {
		return err
	}
	go func() {
		tick := time.NewTicker(time.Duration(client.conf.Client.CheckTime) * time.Second)
		for {
			select {
			case <-tick.C:
				err = client.initProc()
				if err != nil {
					log.Errorf("client set proc, init proc err: %s", err)
				}
				err = client.iniSysctl()
				if err != nil {
					log.Errorf("client set proc, init sysctl err: %s", err)
				}
			case <-client.quit:
				return
			}
		}
	}()
	return nil

}

func (client *Client) initProc() error {
	infoO, infoE, err := utils.Cmd("bash", "-c", "echo 1 > /proc/sys/net/ipv4/conf/all/route_localnet")
	if err != nil {
		log.Errorf("client init proc, enable route local net err: %s, stdout: %s, stderr: %s",
			err, infoO, strings.TrimSuffix(string(infoE), "\n"))
		return err
	}
	log.Debugf("client init proc, enable route local net success, stdout: %s, stderr: %s",
		infoO, strings.TrimSuffix(string(infoE), "\n"))
	return nil
}

func (client *Client) iniSysctl() error {
	infoO, infoE, err := network.EnableFWMark()
	if err != nil {
		log.Errorf("client init proc, enable fwmark err: %s, stdout: %s, stderr: %s",
			err, infoO, strings.TrimSuffix(string(infoE), "\n"))
		return err
	}
	log.Debugf("client init proc, enable fwmark success, stdout: %s, stderr: %s",
		infoO, strings.TrimSuffix(string(infoE), "\n"))
	return nil
}
