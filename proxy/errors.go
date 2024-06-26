/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package proxy

import (
	"errors"
	"strings"
)

var (
	ErrChainExists          = errors.New("iptables: Chain already exists.")
	ErrChainNoMatch         = errors.New("iptables: No chain/target/match by that name.")
	ErrBadRule              = errors.New("iptables: Bad rule (does a matching rule exist in that chain?).")
	ErrUnsupportedLocalMode = errors.New("unsupported local mode")
)

func IsErrChainExists(err string) bool {
	if strings.Contains(err, ErrChainExists.Error()) {
		return true
	}
	return false
}

func IsErrChainNoMatch(err string) bool {
	if strings.Contains(err, ErrChainNoMatch.Error()) {
		return true
	}
	return false
}

func IsErrBadRule(err string) bool {
	if strings.Contains(err, ErrBadRule.Error()) {
		return true
	}
	return false
}
