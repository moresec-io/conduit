/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package errors

import (
	"errors"
	"strings"
)

var (
	ErrChainExists          = errors.New("iptables: Chain already exists.")
	ErrChainNoMatch         = errors.New("iptables: No chain/target/match by that name.")
	ErrBadRule              = errors.New("iptables: Bad rule (does a matching rule exist in that chain?).")
	ErrUnsupportedLocalMode = errors.New("unsupported local mode")

	ErrDuplicatedPeerIndexConfigured = errors.New("duplicated peer index configured")
	ErrPeerIndexNotfound             = errors.New("peer index not found")
	ErrIllegalClientListenAddress    = errors.New("illegal client listen address")

	ErrNoSuchFileOrDirectory = errors.New("o such file or directory") // "no such file or directory" or "No such file or directory"
)

func IsErrChainExists(err error) bool {
	if strings.Contains(err.Error(), ErrChainExists.Error()) {
		return true
	}
	return false
}

func IsErrChainNoMatch(err error) bool {
	if strings.Contains(err.Error(), ErrChainNoMatch.Error()) {
		return true
	}
	return false
}

func IsErrIPSetNoMatch(err error) bool {
	if strings.Contains(err.Error(), "Set") && strings.Contains(err.Error(), "doesn't exist") {
		return true
	}
	return false
}

func IsErrBadRule(err error) bool {
	if strings.Contains(err.Error(), ErrBadRule.Error()) {
		return true
	}
	return false
}

func IsErrNoSuchFileOrDirectory(err error) bool {
	if strings.Contains(err.Error(), ErrNoSuchFileOrDirectory.Error()) {
		return true
	}
	return false
}
