//go:build darwin
// +build darwin

/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */

package network

import (
	"net"
)

func ListIPs() ([]net.IP, error) {
	// Darwin stub implementation
	// Returns empty list on non-Linux platforms
	return []net.IP{}, nil
}

func GetSocketMark(fd uintptr) (uint32, error) {
	// Darwin stub implementation
	// SO_MARK is not available on Darwin
	return 0, nil
}
