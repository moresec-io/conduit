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
	xtables "github.com/singchia/go-xtables"
	"github.com/singchia/go-xtables/iptables"
	"github.com/singchia/go-xtables/pkg/network"
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
	ipt := iptables.NewIPTables()
	// ignore the mark
	exist, err := ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).
		MatchIPv4().MatchProtocol(false, network.ProtocolTCP).MatchMark(false, 5053).
		OptionWait(0).TargetAccept().Check()
	if err != nil {
		log.Errorf("client init tables, check mark err: %s", err)
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).
			MatchIPv4().MatchProtocol(false, network.ProtocolTCP).MatchMark(false, 5053).
			OptionWait(0).TargetAccept().Insert()
		if err != nil {
			log.Errorf("client init tables, insert mark err: %s", err)
			return err
		}
	}

	// create chain
	err = ipt.Table(iptables.TableTypeNat).OptionWait(0).NewChain(ConduitChain)
	if err != nil {
		ce, ok := err.(*xtables.CommandError)
		if !ok || !IsErrChainExists(ce.Message) {
			log.Errorf("client init tables, create conduit chain err: %s", err)
			return err
		}
	}

	// check jump conduit exists, in NAT-PREROUTING
	exist, err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypePREROUTING).MatchInInterface(false, "br-+").
		OptionWait(0).TargetJumpChain(ConduitChain).Check()
	if err != nil {
		log.Errorf("client init tables, check jump conduit chain err: %s", err)
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypePREROUTING).MatchInInterface(false, "br-+").
			OptionWait(0).TargetJumpChain(ConduitChain).Append()
		if err != nil {
			log.Errorf("client init tables, add jump conduit chain err: %s", err)
		}
	}

	// check jump conduit exists, in NAT-OUTPUT
	// src->5013 => src->5052 => ?->5053 => ?->5013 => 如果5013是docker-proxy，那么就会避免重新命中这条iptables
	exist, err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).MatchOutInterface(true, "br-+").
		OptionWait(0).TargetJumpChain(ConduitChain).Check()
	if err != nil {
		log.Errorf("client init tables, check jump conduit chain err: %s", err)
		return err
	}
	if !exist {
		// src->5013 => src->5052 => ?->5053 => ?->5013 => 如果5013是docker-proxy，那么就会避免重新命中这条iptables
		err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).MatchOutInterface(true, "br-+").
			OptionWait(0).TargetJumpChain(ConduitChain).Append()
		if err != nil {
			log.Errorf("client init tables, add jump conduit chain err: %s", err)
			return err
		}
	}

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
			exist, err := ipt.Table(iptables.TableTypeNat).UserDefinedChain(ConduitChain).
				MatchProtocol(false, network.ProtocolIPv4).MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
				OptionWait(0).TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), port)).Check()
			if err != nil {
				log.Errorf("client init tables, check dnat to dst err: %s", err)
				return err
			}
			if !exist {
				err = ipt.Table(iptables.TableTypeNat).UserDefinedChain(ConduitChain).
					MatchProtocol(false, network.ProtocolIPv4).MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
					OptionWait(0).TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), port)).Append()
				if err != nil {
					log.Errorf("client init tables, append dnat to dst err: %s", err)
					return err
				}
			}
		} else {
			// both ip and port, check exist
			exist, err := ipt.Table(iptables.TableTypeNat).UserDefinedChain(ConduitChain).
				MatchProtocol(false, network.ProtocolIPv4).MatchDestination(false, ip).MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
				OptionWait(0).TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), port)).Check()
			if err != nil {
				log.Errorf("client init tables, check dnat to dst err: %s", err)
				return err
			}
			if !exist {
				err = ipt.Table(iptables.TableTypeNat).UserDefinedChain(ConduitChain).
					MatchProtocol(false, network.ProtocolIPv4).MatchDestination(false, ip).MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
					OptionWait(0).TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), port)).Append()
				if err != nil {
					log.Errorf("client init tables, append dnat to dst err: %s", err)
					return err
				}
			}
		}
	}
	return nil
}

func (client *Client) finiTables() {
	ipt := iptables.NewIPTables()
	// delete the mark
	err := ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).
		MatchIPv4().MatchProtocol(false, network.ProtocolTCP).MatchMark(false, 5053).
		OptionWait(0).TargetAccept().Delete()
	if err != nil {
		log.Warnf("client fini tables, delete mark err: %s", err)
	}

	// flush conduit chain
	err = ipt.Table(iptables.TableTypeNat).UserDefinedChain(ConduitChain).
		OptionWait(0).Flush()
	if err != nil {
		log.Warnf("client fini tables, flush conduit chain err: %s", err)
	}

	// delete jump conduit, NAT-PREROUTING
	err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypePREROUTING).
		OptionWait(0).TargetJumpChain(ConduitChain).Delete()
	if err != nil {
		log.Warnf("client fini tables, delete jump conduit chain err: %s", err)
	}

	// delete jump conduit, NAT-OUTPUT
	err = ipt.Table(iptables.TableTypeNat).Chain(iptables.ChainTypeOUTPUT).
		OptionWait(0).TargetJumpChain(ConduitChain).Delete()
	if err != nil {
		log.Warnf("client fini tables, delete jump conduit chain err: %s", err)
	}

	// delete conduit chain
	err = ipt.Table(iptables.TableTypeNat).UserDefinedChain(ConduitChain).
		OptionWait(0).Delete()
	if err != nil {
		log.Warnf("client fini tables, delete conduit chain err: %s", err)
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
