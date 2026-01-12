//go:build darwin
// +build darwin

/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */

package repo

import (
	"net"

	"github.com/jumboframes/armorigo/log"
)

// A wrapper
type ipset struct{}

func (ipset *ipset) InitIPSet() error {
	return nil
}

func (ipset *ipset) AddIPSetIPPort(ip net.IP, port uint16) error {
	return nil
}

func (ipset *ipset) AddIPSetPort(port uint16) error {
	return nil
}

func (ipset *ipset) AddIPSetIP(ip net.IP) error {
	return nil
}

func (ipset *ipset) DelIPSetIPPort(ip net.IP, port uint16) error {
	return nil
}

func (ipset *ipset) DelIPSetPort(port uint16) error {
	return nil
}

func (ipset *ipset) DelIPSetIP(ip net.IP) error {
	return nil
}

func (ipset *ipset) FiniIPSet(level log.Level, prefix string) error {
	return nil
}
