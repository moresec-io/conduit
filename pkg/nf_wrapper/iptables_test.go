/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package nf_wrapper

import "testing"

func Test_AddIptables(t *testing.T) {
	infoO, infoE, err := IptablesRun(
		OptionIptablesTable(IptablesTableNat),
		OptionIptablesChainOperate(IptablesChainAdd),
		OptionIptablesChain(IptablesChainPrerouting),
		OptionIptablesIPv4DstIp("192.168.180.57"),
		OptionIptablesIPv4Proto(IptablesIPv4Tcp),
		OptionIptablesIPv4DstPort(80),
		OptionIptablesJump(IptablesTargetRedirect),
		OptionIptablesJumpSubOptions("--to-port", "80"))
	if err != nil {
		t.Errorf("exec err: %s", err)
		return
	}
	t.Logf("infoO: %v, infoE: %v", infoO, infoE)
}

func Test_DelIptables(t *testing.T) {
	infoO, infoE, err := IptablesRun(
		OptionIptablesTable(IptablesTableNat),
		OptionIptablesChainOperate(IptablesChainDel),
		OptionIptablesChain(IptablesChainPrerouting),
		OptionIptablesIPv4DstIp("192.168.180.57"),
		OptionIptablesIPv4Proto(IptablesIPv4Tcp),
		OptionIptablesIPv4DstPort(80),
		OptionIptablesJump(IptablesTargetRedirect),
		OptionIptablesJumpSubOptions("--to-port", "80"))
	if err != nil {
		t.Errorf("exec err: %s", err)
		return
	}
	t.Logf("infoO: %v, infoE: %v", infoO, infoE)
}
