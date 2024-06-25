/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package proxy

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/jumboframes/armorigo/log"
	nfw "github.com/moresec-io/conduit/pkg/nf_wrapper"
)

func (client *Client) setTables() error {
	client.finiTables()
	err := client.initTables()
	if err != nil {
		return err
	}
	go func() {
		tick := time.NewTicker(time.Duration(client.conf.Client.Proxy.CheckTime) * time.Second)
		for {
			select {
			case <-tick.C:
				err = client.initTables()
				if err != nil {
					log.Errorf("Client::setTables | init tables err: %s", err)
				}
			case <-client.quit:
				return
			}
		}
	}()
	return nil
}

func (client *Client) initTables() error {
	// ignore the mark
	infoO, infoE, err := nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainCheck),
		nfw.OptionIptablesChain(nfw.IptablesChainOutput),
		nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
		nfw.OptionIptablesMatch("mark"),
		nfw.OptionIptablesMatchSubOptions("--mark", "5053"),
		nfw.OptionIptablesJump(nfw.IptablesTargetAccept),
	)
	if err != nil && !IsErrChainNoMatch(infoE) && !IsErrBadRule(infoE) {
		log.Errorf("Client::SetTables | check mark err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
		return err
	}
	if IsErrChainNoMatch(infoE) || IsErrBadRule(infoE) {
		infoO, infoE, err := nfw.IptablesRun(
			nfw.OptionIptablesWait(),
			nfw.OptionIptablesTable(nfw.IptablesTableNat),
			nfw.OptionIptablesChainOperate(nfw.IptablesChainI),
			nfw.OptionIptablesChain(nfw.IptablesChainOutput),
			nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
			nfw.OptionIptablesMatch("mark"),
			nfw.OptionIptablesMatchSubOptions("--mark", "5053"),
			nfw.OptionIptablesJump(nfw.IptablesTargetAccept),
		)
		if err != nil {
			log.Errorf("Client::SetTables | insert mark err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
			return err
		}
	}

	// check chain exists
	infoO, infoE, err = nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainNew),
		nfw.OptionIptablesChain(MsProxyChain),
	)
	if err != nil && !IsErrChainExists(infoE) {
		log.Errorf("Client::SetTables | new chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
		return err
	}
	// check chain at nat-prerouting
	for _, bridge := range localBridges() {
		infoO, infoE, err = nfw.IptablesRun(
			nfw.OptionIptablesWait(),
			nfw.OptionIptablesTable(nfw.IptablesTableNat),
			nfw.OptionIptablesChainOperate(nfw.IptablesChainCheck),
			nfw.OptionIptablesChain(nfw.IptablesChainPrerouting),
			nfw.OptionIptablesInIf(bridge),
			nfw.OptionIptablesJump(MsProxyChain),
		)
		if err != nil && !IsErrChainNoMatch(infoE) {
			log.Errorf("Client::SetTables | check output chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
			return err
		}
		if IsErrChainNoMatch(infoE) {
			infoO, infoE, err = nfw.IptablesRun(
				nfw.OptionIptablesWait(),
				nfw.OptionIptablesTable(nfw.IptablesTableNat),
				nfw.OptionIptablesChainOperate(nfw.IptablesChainAdd),
				nfw.OptionIptablesChain(nfw.IptablesChainPrerouting),
				nfw.OptionIptablesInIf(bridge),
				nfw.OptionIptablesJump(MsProxyChain),
			)
			if err != nil {
				log.Errorf("Client::SetTables | add output chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
				return err
			}
		}
	}
	// check chain at nat-output
	infoO, infoE, err = nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainCheck),
		nfw.OptionIptablesChain(nfw.IptablesChainOutput),
		nfw.OptionIptablesJump(MsProxyChain),
	)
	if err != nil && !IsErrChainNoMatch(infoE) {
		log.Errorf("Client::SetTables | check output chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
		return err
	}
	if IsErrChainNoMatch(infoE) {
		infoO, infoE, err = nfw.IptablesRun(
			nfw.OptionIptablesWait(),
			nfw.OptionIptablesTable(nfw.IptablesTableNat),
			nfw.OptionIptablesChainOperate(nfw.IptablesChainAdd),
			nfw.OptionIptablesChain(nfw.IptablesChainOutput),
			nfw.OptionIptablesJump(MsProxyChain),
		)
		if err != nil {
			log.Errorf("Client::SetTables | add output chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
			return err
		}
	}

	// real maps
	for _, transfer := range client.conf.Client.Proxy.Transfers {
		transferIpPort := strings.Split(transfer.Dst, ":")
		ip := transferIpPort[0]
		port, err := strconv.Atoi(transferIpPort[1])
		if err != nil {
			continue
		}
		if ip == "" {
			// only port
			infoO, infoE, err := nfw.IptablesRun(
				nfw.OptionIptablesWait(),
				nfw.OptionIptablesTable(nfw.IptablesTableNat),
				nfw.OptionIptablesChainOperate(nfw.IptablesChainCheck),
				nfw.OptionIptablesChain(MsProxyChain),
				nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
				nfw.OptionIptablesIPv4DstPort(uint32(port)),
				nfw.OptionIptablesJump(nfw.IptablesTargetRedirect),
				nfw.OptionIptablesJumpSubOptions("--to-ports", strconv.Itoa(client.port)),
			)
			if err == nil || (err != nil && IsErrChainExists(infoE)) {
				continue
			}
			if err != nil && !IsErrChainNoMatch(infoE) {
				log.Errorf("Client::SetTables | check chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
				return err
			}

			infoO, infoE, err = nfw.IptablesRun(
				nfw.OptionIptablesWait(),
				nfw.OptionIptablesTable(nfw.IptablesTableNat),
				nfw.OptionIptablesChainOperate(nfw.IptablesChainAdd),
				nfw.OptionIptablesChain(MsProxyChain),
				nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
				nfw.OptionIptablesIPv4DstPort(uint32(port)),
				nfw.OptionIptablesJump(nfw.IptablesTargetRedirect),
				nfw.OptionIptablesJumpSubOptions("--to-ports", strconv.Itoa(client.port)),
			)
			if err != nil {
				if IsErrChainExists(infoE) {
					continue
				}
				log.Errorf("Client::SetTables | add on chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
				return err
			}

		} else {
			// both ip and port
			infoO, infoE, err := nfw.IptablesRun(
				nfw.OptionIptablesWait(),
				nfw.OptionIptablesTable(nfw.IptablesTableNat),
				nfw.OptionIptablesChainOperate(nfw.IptablesChainCheck),
				nfw.OptionIptablesChain(MsProxyChain),
				nfw.OptionIptablesIPv4DstIp(ip),
				nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
				nfw.OptionIptablesIPv4DstPort(uint32(port)),
				nfw.OptionIptablesJump(nfw.IptablesTargetRedirect),
				nfw.OptionIptablesJumpSubOptions("--to-ports", strconv.Itoa(client.port)),
			)
			if err == nil || (err != nil && IsErrChainExists(infoE)) {
				continue
			}
			if err != nil && !IsErrChainNoMatch(infoE) {
				log.Errorf("Client::SetTables | check chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
				return err
			}

			infoO, infoE, err = nfw.IptablesRun(
				nfw.OptionIptablesWait(),
				nfw.OptionIptablesTable(nfw.IptablesTableNat),
				nfw.OptionIptablesChainOperate(nfw.IptablesChainAdd),
				nfw.OptionIptablesChain(MsProxyChain),
				nfw.OptionIptablesIPv4DstIp(ip),
				nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
				nfw.OptionIptablesIPv4DstPort(uint32(port)),
				nfw.OptionIptablesJump(nfw.IptablesTargetRedirect),
				nfw.OptionIptablesJumpSubOptions("--to-ports", strconv.Itoa(client.port)),
			)
			if err != nil {
				if IsErrChainExists(infoE) {
					continue
				}
				log.Errorf("Client::SetTables | add on chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
				return err
			}
		}
	}
	return nil
}

func (client *Client) finiTables() {
	infoO, infoE, err := nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainDel),
		nfw.OptionIptablesChain(nfw.IptablesChainOutput),
		nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
		nfw.OptionIptablesMatch("mark"),
		nfw.OptionIptablesMatchSubOptions("--mark", "5053"),
		nfw.OptionIptablesJump(nfw.IptablesTargetAccept),
	)
	if err != nil {
		log.Errorf("Client::finiTables | delete mark err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
	}
	infoO, infoE, err = nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainFlush),
		nfw.OptionIptablesChain(MsProxyChain),
	)
	if err != nil {
		log.Errorf("Client::finiTables | flush chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
	}

	infoO, infoE, err = nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainDel),
		nfw.OptionIptablesChain(nfw.IptablesChainOutput),
		nfw.OptionIptablesJump(MsProxyChain),
	)
	if err != nil {
		log.Errorf("Client::finiTables | del output chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
	}

	infoO, infoE, err = nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainX),
		nfw.OptionIptablesChain(MsProxyChain),
	)
	if err != nil {
		log.Errorf("Client::finiTables | del chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
	}
}

func localBridges() []string {
	bridges := []string{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return bridges
	}

	for _, iface := range ifaces {
		if strings.Contains(iface.Name, "br-") {
			bridges = append(bridges, iface.Name)
		}
	}
	return bridges
}
