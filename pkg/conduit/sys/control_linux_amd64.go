//go:build linux && amd64
// +build linux,amd64

/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */

package sys

import (
	"syscall"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/conduit/config"

	"golang.org/x/sys/unix"
)

func Control(network, address string, conn syscall.RawConn) error {
	var operr, err error
	err = conn.Control(func(fd uintptr) {
		operr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
		if operr != nil {
			return
		}
		operr = syscall.SetsockoptInt(int(fd), unix.SOL_SOCKET, syscall.SO_MARK, config.MarkIgnoreOurself)
		if operr != nil {
			return
		}
	})
	if err != nil {
		return err
	}
	if operr != nil {
		log.Errorf("control | set sock opt err: %s", operr)
		return operr
	}
	return nil
}
