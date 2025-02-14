/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package client

import (
	"strconv"
	"strings"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/conduit/config"
	"github.com/moresec-io/conduit/pkg/conduit/errors"
	xtables "github.com/singchia/go-xtables"
	"github.com/singchia/go-xtables/iptables"
	"github.com/singchia/go-xtables/pkg/network"
)

func (client *Client) setTables() error {
	err := client.initTables()
	if err != nil {
		log.Errorf("client set tables, init tables err: %s", err)
		return err
	}
	go func() {
		tick := time.NewTicker(time.Duration(client.conf.Client.CheckTime) * time.Second)
		for {
			select {
			case <-tick.C:
				err = client.initTables()
				if err != nil {
					log.Errorf("client set tables, init tables err: %s", err)
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

	// ignore manager connection
	if client.conf.Manager.Enable {
		for _, address := range client.conf.Manager.Dial.Addresses {
			ipport := strings.Split(address, ":")
			if len(ipport) != 2 {
				continue
			}
			port, err := strconv.Atoi(ipport[1])
			if err != nil {
				continue
			}
			exist, err := ipt.Table(iptables.TableTypeNat).
				Chain(iptables.ChainTypeOUTPUT).
				MatchIPv4().
				MatchProtocol(false, network.ProtocolTCP).
				MatchDestination(false, ipport[0]).
				MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
				OptionWait(0).
				TargetAccept().
				Check()
			if err != nil {
				log.Errorf("client init tables, check manager addr err: %s", strings.TrimSuffix(err.Error(), "\n"))
				return err
			}
			if !exist {
				err = ipt.Table(iptables.TableTypeNat).
					Chain(iptables.ChainTypeOUTPUT).
					MatchIPv4().
					MatchProtocol(false, network.ProtocolTCP).
					MatchDestination(false, ipport[0]).
					MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
					OptionWait(0).
					TargetAccept().
					Insert()
				if err != nil {
					log.Errorf("client init tables, insert manager addr err: %s", strings.TrimSuffix(err.Error(), "\n"))
					return err
				}
			}
		}
	}

	// ignore ourself connection by setting mark
	exist, err := ipt.Table(iptables.TableTypeNat).
		Chain(iptables.ChainTypeOUTPUT).
		MatchIPv4().
		MatchProtocol(false, network.ProtocolTCP).
		MatchMark(false, config.MarkIgnoreOurself).
		OptionWait(0).
		TargetAccept().
		Check()
	if err != nil {
		log.Errorf("client init tables, check mark err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).
			Chain(iptables.ChainTypeOUTPUT).
			MatchIPv4().
			MatchProtocol(false, network.ProtocolTCP).
			MatchMark(false, config.MarkIgnoreOurself).
			OptionWait(0).
			TargetAccept().
			Insert()
		if err != nil {
			log.Errorf("client init tables, insert mark err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}

	// create conduit chain
	err = ipt.Table(iptables.TableTypeNat).
		OptionWait(0).
		NewChain(ConduitChain)
	if err != nil {
		_, ok := err.(*xtables.CommandError)
		if !ok || !errors.IsErrChainExists(err) {
			log.Errorf("client init tables, create conduit chain err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}

	// check jump conduit exists, in NAT-PREROUTING, for bridged traffic
	exist, err = ipt.Table(iptables.TableTypeNat).
		Chain(iptables.ChainTypePREROUTING).
		MatchInInterface(false, "br+").
		OptionWait(0).
		TargetJumpChain(ConduitChain).
		Check()
	if err != nil && !errors.IsErrChainNoMatch(err) { // TODO PR to go-xtables
		log.Errorf("client init tables, check jump conduit chain err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).
			Chain(iptables.ChainTypePREROUTING).
			MatchInInterface(false, "br+").
			OptionWait(0).
			TargetJumpChain(ConduitChain).Append()
		if err != nil {
			log.Errorf("client init tables, add jump conduit chain err: %s", strings.TrimSuffix(err.Error(), "\n"))
		}
	}
	// check jump conduit exists, in NAT-OUTPUT, for local traffic
	// src->5013 => src->5052 => ?->5053 => ?->5013 => ...
	// if the port 5013 belongs other proxy like docker-prory,
	// this rule would avoid dead loop
	exist, err = ipt.Table(iptables.TableTypeNat).
		Chain(iptables.ChainTypeOUTPUT).
		MatchOutInterface(true, "br+").
		OptionWait(0).
		TargetJumpChain(ConduitChain).
		Check()
	if err != nil && !errors.IsErrChainNoMatch(err) {
		log.Errorf("client init tables, check jump conduit chain err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).
			Chain(iptables.ChainTypeOUTPUT).
			MatchOutInterface(true, "br+").
			OptionWait(0).
			TargetJumpChain(ConduitChain).Append()
		if err != nil {
			log.Errorf("client init tables, add jump conduit chain err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}

	userDefined := iptables.ChainTypeUserDefined
	userDefined.SetName(ConduitChain)

	// add ipset match mark, ipport > port > ip
	// ip set match and set mark
	exist, err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIP, iptables.FlagDst)).OptionWait(0).
		TargetMark(iptables.WithTargetMarkSet(config.MarkIpsetIP)).
		Check()
	if err != nil && !errors.IsErrChainNoMatch(err) {
		log.Errorf("client init tables, check mark err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).
			Chain(userDefined).
			MatchProtocol(false, network.ProtocolTCP).
			MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIP, iptables.FlagDst)).
			OptionWait(0).
			TargetMark(iptables.WithTargetMarkSet(config.MarkIpsetIP)).
			Insert()
		if err != nil {
			log.Errorf("client init tables, insert mark err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}
	// port set match and set mark
	exist, err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetPort, iptables.FlagDst)).
		OptionWait(0).
		TargetMark(iptables.WithTargetMarkSet(config.MarkIpsetPort)).
		Check()
	if err != nil && !errors.IsErrChainNoMatch(err) {
		log.Errorf("client init tables, check mark err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).
			Chain(userDefined).
			MatchProtocol(false, network.ProtocolTCP).
			MatchSet(iptables.WithMatchSetName(false, ConduitIPSetPort, iptables.FlagDst)).
			OptionWait(0).
			TargetMark(iptables.WithTargetMarkSet(config.MarkIpsetPort)).
			Insert()
		if err != nil {
			log.Errorf("client init tables, insert mark err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}
	// ipport set match and set mark
	exist, err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIPPort, iptables.FlagDst, iptables.FlagDst)).
		OptionWait(0).
		TargetMark(iptables.WithTargetMarkSet(config.MarkIpsetIPPort)).
		Check()
	if err != nil && !errors.IsErrChainNoMatch(err) {
		log.Errorf("client init tables, check ipport target mark err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).
			Chain(userDefined).
			MatchProtocol(false, network.ProtocolTCP).
			MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIPPort, iptables.FlagDst, iptables.FlagDst)).
			OptionWait(0).
			TargetMark(iptables.WithTargetMarkSet(config.MarkIpsetIPPort)).
			Insert()
		if err != nil {
			log.Errorf("client init tables, insert ipport target mark err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}

	// add ipset match and dnat, ipport > port > ip
	// dnat ip port
	exist, err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIPPort, iptables.FlagDst, iptables.FlagDst)).
		OptionWait(0).
		TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).
		Check()
	if err != nil && !errors.IsErrChainNoMatch(err) {
		log.Errorf("client init tables, check ipport dnat to dst err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).
			Chain(userDefined).
			MatchProtocol(false, network.ProtocolTCP).
			MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIPPort, iptables.FlagDst, iptables.FlagDst)).
			OptionWait(0).
			TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).
			Append()
		if err != nil {
			log.Errorf("client init tables, append ipport dnat to dst err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}
	// dnat port
	exist, err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetPort, iptables.FlagDst)).
		OptionWait(0).
		TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).
		Check()
	if err != nil && !errors.IsErrChainNoMatch(err) {
		log.Errorf("client init tables, check port dnat to dst err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).
			Chain(userDefined).
			MatchProtocol(false, network.ProtocolTCP).
			MatchSet(iptables.WithMatchSetName(false, ConduitIPSetPort, iptables.FlagDst)).
			OptionWait(0).
			TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).
			Append()
		if err != nil {
			log.Errorf("client init tables, append port dnat to dst err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}
	// dnat ip
	exist, err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIP, iptables.FlagDst)).
		OptionWait(0).
		TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).
		Check()
	if err != nil && !errors.IsErrChainNoMatch(err) {
		log.Errorf("client init tables, check ipport dnat to dst err: %s", strings.TrimSuffix(err.Error(), "\n"))
		return err
	}
	if !exist {
		err = ipt.Table(iptables.TableTypeNat).
			Chain(userDefined).
			MatchProtocol(false, network.ProtocolTCP).
			MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIP, iptables.FlagDst)).
			OptionWait(0).
			TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).
			Append()
		if err != nil {
			log.Errorf("client init tables, append ipport dnat to dst err: %s", strings.TrimSuffix(err.Error(), "\n"))
			return err
		}
	}
	return nil
}

func (client *Client) finiTables(level log.Level, prefix string) {
	ipt := iptables.NewIPTables()

	// delete ignore manager connection
	if client.conf.Manager.Enable {
		for _, address := range client.conf.Manager.Dial.Addresses {
			ipport := strings.Split(address, ":")
			if len(ipport) != 2 {
				continue
			}
			port, err := strconv.Atoi(ipport[1])
			if err != nil {
				continue
			}
			err = ipt.Table(iptables.TableTypeNat).
				Chain(iptables.ChainTypeOUTPUT).
				MatchIPv4().
				MatchProtocol(false, network.ProtocolTCP).
				MatchDestination(false, ipport[0]).
				MatchTCP(iptables.WithMatchTCPDstPort(false, port)).
				OptionWait(0).
				TargetAccept().
				Delete()
			if err != nil {
				log.Printf(level, "%s, delete manager addr err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
			}
		}
	}

	// delete the mark
	err := ipt.Table(iptables.TableTypeNat).
		Chain(iptables.ChainTypeOUTPUT).
		MatchIPv4().
		MatchProtocol(false, network.ProtocolTCP).
		MatchMark(false, config.MarkIgnoreOurself).
		OptionWait(0).
		TargetAccept().
		Delete()
	if err != nil && !errors.IsErrBadRule(err) {
		log.Printf(level, "%s, delete mark err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}

	userDefined := iptables.ChainTypeUserDefined
	userDefined.SetName(ConduitChain)
	// delete marks
	// delete ip mark
	err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIP, iptables.FlagDst)).
		OptionWait(0).
		TargetMark(iptables.WithTargetMarkSet(config.MarkIpsetIP)).
		Delete()
	if err != nil && !errors.IsErrIPSetNoMatch(err) && !errors.IsErrChainNoMatch(err) {
		log.Printf(level, "%s, delete ip ipset mark err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}
	// delete port mark
	err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetPort, iptables.FlagDst)).
		OptionWait(0).
		TargetMark(iptables.WithTargetMarkSet(config.MarkIpsetPort)).
		Delete()
	if err != nil && !errors.IsErrIPSetNoMatch(err) && !errors.IsErrChainNoMatch(err) {
		log.Printf(level, "%s, delete port ipset mark err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}
	err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIPPort, iptables.FlagDst, iptables.FlagDst)).
		OptionWait(0).
		TargetMark(iptables.WithTargetMarkSet(config.MarkIpsetIPPort)).
		Delete()
	if err != nil && !errors.IsErrIPSetNoMatch(err) && !errors.IsErrChainNoMatch(err) {
		log.Printf(level, "%s, delete ipport ipset mark err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}
	// delete dnats
	// dnat port
	err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetPort, iptables.FlagDst)).
		OptionWait(0).
		TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).
		Delete()
	if err != nil && !errors.IsErrIPSetNoMatch(err) && !errors.IsErrChainNoMatch(err) {
		log.Printf(level, "%s, delete dnat err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}
	// dnat ip port
	err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIPPort, iptables.FlagDst, iptables.FlagDst)).
		OptionWait(0).
		TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).
		Delete()
	if err != nil && !errors.IsErrIPSetNoMatch(err) && !errors.IsErrChainNoMatch(err) {
		log.Printf(level, "%s, delete dnat err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}
	// dnat ip
	err = ipt.Table(iptables.TableTypeNat).
		Chain(userDefined).
		MatchProtocol(false, network.ProtocolTCP).
		MatchSet(iptables.WithMatchSetName(false, ConduitIPSetIP, iptables.FlagDst)).
		OptionWait(0).
		TargetDNAT(iptables.WithTargetDNATToAddr(network.ParseIP("127.0.0.1"), client.port)).
		Delete()
	if err != nil && !errors.IsErrIPSetNoMatch(err) && !errors.IsErrChainNoMatch(err) {
		log.Printf(level, "%s, delete dnat err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}

	// flush conduit chain
	err = ipt.Table(iptables.TableTypeNat).
		UserDefinedChain(ConduitChain).
		OptionWait(0).
		Flush()
	if err != nil && !errors.IsErrChainNoMatch(err) {
		log.Printf(level, "%s, flush conduit chain err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}

	// delete jump conduit, NAT-PREROUTING
	err = ipt.Table(iptables.TableTypeNat).
		Chain(iptables.ChainTypePREROUTING).
		OptionWait(0).
		TargetJumpChain(ConduitChain).
		Delete()
	if err != nil && !errors.IsErrChainNoMatch(err) && !errors.IsErrNoSuchFileOrDirectory(err) {
		log.Printf(level, "%s, delete jump conduit chain err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}
	// delete jump conduit, NAT-OUTPUT
	err = ipt.Table(iptables.TableTypeNat).
		Chain(iptables.ChainTypeOUTPUT).
		OptionWait(0).
		TargetJumpChain(ConduitChain).
		Delete()
	if err != nil && !errors.IsErrChainNoMatch(err) && !errors.IsErrNoSuchFileOrDirectory(err) {
		log.Printf(level, "%s, delete jump conduit chain err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}

	// delete conduit chain
	err = ipt.Table(iptables.TableTypeNat).
		UserDefinedChain(ConduitChain).
		OptionWait(0).
		Delete()
	if err != nil && !errors.IsErrBadRule(err) {
		log.Printf(level, "%s, delete conduit chain err: %s", prefix, strings.TrimSuffix(err.Error(), "\n"))
	}
}
