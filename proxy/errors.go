/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Austin Zhai
 * All rights reserved.
 */
package proxy

import (
	"errors"
	"strings"
)

var (
	ErrChainExists  = errors.New("iptables: Chain already exists.")
	ErrChainNoMatch = errors.New("iptables: No chain/target/match by that name.")
	ErrBadRule      = errors.New("iptables: Bad rule (does a matching rule exist in that chain?).")
)

func IsErrChainExists(err []byte) bool {
	if strings.Contains(string(err), ErrChainExists.Error()) {
		return true
	}
	return false
}

func IsErrChainNoMatch(err []byte) bool {
	if strings.Contains(string(err), ErrChainNoMatch.Error()) {
		return true
	}
	return false
}

func IsErrBadRule(err []byte) bool {
	if strings.Contains(string(err), ErrBadRule.Error()) {
		return true
	}
	return false
}
