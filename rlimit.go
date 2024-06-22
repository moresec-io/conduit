/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package conduit

import (
	"syscall"
)

func SetRLimit(fileLimit uint64) error {
	var rLimit syscall.Rlimit
	rLimit.Cur = fileLimit
	rLimit.Max = fileLimit
	err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	return err
}
