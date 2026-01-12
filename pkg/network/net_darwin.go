//go:build darwin
// +build darwin

/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */

package network

func EnableFWMark() ([]byte, []byte, error) {
	// Darwin stub implementation
	// net.ipv4.tcp_fwmark_accept is Linux-specific
	return nil, nil, nil
}
