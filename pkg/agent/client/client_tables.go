/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package client

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/agent/errors"
	xtables "github.com/singchia/go-xtables"
	"github.com/singchia/go-xtables/iptables"
	"github.com/singchia/go-xtables/pkg/network"
)

func (client *Client) setTables() error {
	client.finiTables("flush tables before init")
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
	ipt := iptables.NewIPTables()
	// ignore the mark
	exist, err := ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).
		MatchIPv4().MatchProtocol(false, network.ProtocolTCP).MatchMark(false, 5053).
		OptionWait(0).TargetAccept().Check()
	if err != nil {
		log.Errorf("client init tables, check mark err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).
			MatchIPv4().MatchProtocol(false, network.ProtocolTCP).MatchMark(false, 5053).
			OptionWait(0).TargetAccept().Insert()
		if err != nil {
			log.Errorf("client init tables, insert mark err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}

	// create chain
	err = ipt.Table(iptables.TableTypeNat).OptionWait(0).NewChain(ConduitChain)
	if err != nil {
		ce, ok := err.(*xtables.CommandError)
		if !ok || !errors.IsErrChainExists(ce.Message) {
			log.Errorf("client init tables, create conduit chain err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}

	// check jump conduit exists, in NAT-PREROUTING
	exist, err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypePREROUTING).MatchInInterface(false, "br-+").
		OptionWait(0).TargetJumpChain(ConduitChain).Check()
	if err != nil {
		log.Errorf("client init tables, check jump conduit chain err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypePREROUTING).MatchInInterface(false, "br-+").
			OptionWait(0).TargetJumpChain(ConduitChain).Append()
		if err != nil {
			log.Errorf("client init tables, add jump conduit chain err: %s", strings.TrimSuffix(err.Error(), "\n"))
		}
	}

	// check jump conduit exists, in NAT-OUTPUT
	// src->5013 => src->5052 => ?->5053 => ?->5013 => 如果5013是docker-proxy，那么就会避免重新命中这条iptables
	exist, err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).MatchOutInterface(true, "br-+").
		OptionWait(0).TargetJumpChain(ConduitChain).Check()
	if err != nil {
		log.Errorf("client init tables, check jump conduit chain err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		// src->5013 => src->5052 => ?->5053 => ?->5013 => 如果5013是docker-proxy，那么就会避免重新命中这条iptables
		err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).MatchOutInterface(true, "br-+").
			OptionWait(0).TargetJumpChain(ConduitChain).Append()
		if err != nil {
			log.Errorf("client init tables, add jump conduit chain err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}

	userDefined := iptables.ChainTypeUserDefined
	userDefined.SetName(ConduitChain)

	// do real maps
	for _, transfer := range client.conf.Client.Proxy.Transfers {
		transferIpPort := strings.Split(transfer.Dst, ":")
		ip := transferIpPort[0]
		port, err := strconv.Atoi(transferIpPort[1])
		if err != nil {
			continue
		}
		if ip == "" {
			// only port, check exist
			exist, err := ipt.Table(iptables.TableTypeNat).Chain(userDefined).
				MatchProtocol(false, network.ProtocolTCP).MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
				OptionWait(0).TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).Check()
			if err != nil {
				log.Errorf("client init tables, check dnat to dst err: %s", strings.TrimSuffix(err.Error(), "\n"))
				return err
			}
			if !exist {
				err = ipt.Table(iptables.TableTypeNat).Chain(userDefined).
					MatchProtocol(false, network.ProtocolTCP).MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
					OptionWait(0).TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).Append()
				if err != nil {
					log.Errorf("client init tables, append dnat to dst err: %s", strings.TrimSuffix(err.Error(), "\n"))
					return err
				}
			}
		} else {
			// both ip and port, check exist
			exist, err := ipt.Table(iptables.TableTypeNat).Chain(userDefined).
				MatchProtocol(false, network.ProtocolTCP).MatchDestination(false, ip).MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
				OptionWait(0).TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).Check()
			if err != nil {
				log.Errorf("client init tables, check dnat to dst err: %s", strings.TrimSuffix(err.Error(), "\n"))
				return err
			}
			if !exist {
				err = ipt.Table(iptables.TableTypeNat).Chain(userDefined).
					MatchProtocol(false, network.ProtocolTCP).MatchDestination(false, ip).MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
					OptionWait(0).TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).Append()
				if err != nil {
					log.Errorf("client init tables, append dnat to dst err: %s", strings.TrimSuffix(err.Error(), "\n"))
					return err
				}
			}
		}
	}
	return nil
}

func (client *Client) finiTables(prefix string) {
	ipt := iptables.NewIPTables()
	// delete the mark
	err := ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).
		MatchIPv4().MatchProtocol(false, network.ProtocolTCP).MatchMark(false, 5053).
		OptionWait(0).TargetAccept().Delete()
	if err != nil {
		log.Debugf("%s, delete mark err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}

	// flush conduit chain
	err = ipt.Table(iptables.TableTypeNat).UserDefinedChain(ConduitChain).
		OptionWait(0).Flush()
	if err != nil {
		log.Debugf("%s, flush conduit chain err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}

	// delete jump conduit, NAT-PREROUTING
	err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypePREROUTING).
		OptionWait(0).TargetJumpChain(ConduitChain).Delete()
	if err != nil {
		log.Debugf("%s, delete jump conduit chain err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}

	// delete jump conduit, NAT-OUTPUT
	err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).
		OptionWait(0).TargetJumpChain(ConduitChain).Delete()
	if err != nil {
		log.Debugf("%s, delete jump conduit chain err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}

	// delete conduit chain
	err = ipt.Table(iptables.TableTypeNat).UserDefinedChain(ConduitChain).
		OptionWait(0).Delete()
	if err != nil {
		log.Debugf("%s, delete conduit chain err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
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
