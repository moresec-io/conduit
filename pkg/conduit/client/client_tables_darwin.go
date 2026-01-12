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
	"github.com/jumboframes/armorigo/log"
)

func (client *Client) setTables() error {
	// Darwin stub implementation
	// iptables is Linux-specific
	return nil
}

func (client *Client) initTables() error {
	// Darwin stub implementation
	// iptables is Linux-specific
	return nil
}

func (client *Client) finiTables(level log.Level, prefix string) {
	// Darwin stub implementation
	// iptables is Linux-specific
}
