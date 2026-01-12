//go:build darwin
// +build darwin

/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */

package client

import (
	"github.com/moresec-io/conduit/pkg/network"
)

func (client *Client) setProc() error {
	// Darwin stub implementation
	// /proc filesystem operations are Linux-specific
	return nil
}

func (client *Client) initProc() error {
	// Darwin stub implementation
	// /proc/sys/net/ipv4/conf/all/route_localnet is Linux-specific
	return nil
}

func (client *Client) iniSysctl() error {
	// Darwin stub implementation
	// Linux-specific sysctl operations
	_, _, err := network.EnableFWMark()
	return err
}
