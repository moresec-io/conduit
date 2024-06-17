//go:build darwin && amd64
// +build darwin,amd64

/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */

package proxy

import "syscall"

func control(network, address string, conn syscall.RawConn) error {
	return nil
}
