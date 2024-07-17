/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package client

import (
	"net"

	"github.com/jumboframes/armorigo/log"
	"github.com/vishvananda/netlink"
)

func (client *Client) setIPSet() error {
	err := client.initIPSet()
	if err != nil {
		return err
	}
	return nil
}

func (client *Client) initIPSet() error {
	err := netlink.IpsetCreate(ConduitIPSetPort, "bitmap:port", netlink.IpsetCreateOptions{
		PortFrom: 0,
		PortTo:   65535,
	})
	if err != nil {
		log.Errorf("client init port ipset, init err: %s", err)
		return err
	}
	err = netlink.IpsetCreate(ConduitIPSetIPPort, "hash:ip,port", netlink.IpsetCreateOptions{
		PortFrom: 0,
		PortTo:   65535,
	})
	if err != nil {
		log.Errorf("client init ipport ipset, init err: %s", err)
		return err
	}
	err = netlink.IpsetCreate(ConduitIPSetIP, "hash:ip", netlink.IpsetCreateOptions{})
	if err != nil {
		log.Errorf("client init ip ipset, init err: %s", err)
		return err
	}
	return nil
}

func (client *Client) AddIPSetIPPort(ip net.IP, port uint16) error {
	err := netlink.IpsetAdd(ConduitIPSetIPPort, &netlink.IPSetEntry{
		IP:   ip,
		Port: &port,
	})
	if err != nil {
		log.Errorf("client add ipset ip: %s, port: %d err: %s", ip, port, err)
	}
	return err
}

func (client *Client) AddIPSetPort(port uint16) error {
	err := netlink.IpsetAdd(ConduitIPSetPort, &netlink.IPSetEntry{
		Port: &port,
	})
	if err != nil {
		log.Errorf("client add ipset port: %d err: %s", port, err)
	}
	return err
}

func (client *Client) AddIPSetIP(ip net.IP) error {
	err := netlink.IpsetAdd(ConduitIPSetIP, &netlink.IPSetEntry{
		IP: ip,
	})
	if err != nil {
		log.Errorf("client del ip: %s err: %s", ip, err)
	}
	return err
}

func (client *Client) DelIPSetIPPort(ip net.IP, port uint16) error {
	err := netlink.IpsetDel(ConduitIPSetIPPort, &netlink.IPSetEntry{
		IP:   ip,
		Port: &port,
	})
	if err != nil {
		log.Errorf("client del ipset ip: %s, port: %d err: %s", ip, port, err)
	}
	return err
}

func (client *Client) DelIPSetPort(port uint16) error {
	err := netlink.IpsetDel(ConduitIPSetPort, &netlink.IPSetEntry{
		Port: &port,
	})
	if err != nil {
		log.Errorf("client del ipset port: %d err: %s", port, err)
	}
	return err
}

func (client *Client) DelIPSetIP(ip net.IP) error {
	err := netlink.IpsetDel(ConduitIPSetIP, &netlink.IPSetEntry{
		IP: ip,
	})
	if err != nil {
		log.Errorf("client del ip: %s err: %s", ip, err)
	}
	return err
}

func (client *Client) finiIPSet(level log.Level, prefix string) error {
	// flush
	err := netlink.IpsetFlush(ConduitIPSetIPPort)
	if err != nil {
		log.Printf(level, "%s, flush ipset: %s err: %s", prefix, ConduitIPSetPort, err)
	}
	err = netlink.IpsetFlush(ConduitIPSetPort)
	if err != nil {
		log.Printf(level, "%s, flush ipset: %s err: %s", prefix, ConduitIPSetPort, err)
	}
	err = netlink.IpsetFlush(ConduitIPSetIP)
	if err != nil {
		log.Printf(level, "%s, flush ipset: %s err: %s", prefix, ConduitIPSetIP, err)
	}

	// destroy
	err = netlink.IpsetDestroy(ConduitIPSetPort)
	if err != nil {
		log.Printf(level, "%s, destroy ipset: %s err: %s", prefix, ConduitIPSetPort, err)
	}
	err = netlink.IpsetDestroy(ConduitIPSetIPPort)
	if err != nil {
		log.Printf(level, "%s, destroy ipset: %s err: %s", prefix, ConduitIPSetIPPort, err)
	}
	err = netlink.IpsetDestroy(ConduitIPSetIP)
	if err != nil {
		log.Printf(level, "%s, destroy ipset: %s err: %s", prefix, ConduitIPSetIP, err)
	}
	return nil
}
