/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Austin Zhai
 * All rights reserved.
 */
// +build darwin,amd64

package proxy

import "syscall"

func control(network, address string, conn syscall.RawConn) error {
	return nil
}
