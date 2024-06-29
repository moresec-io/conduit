package client

import (
	"net"

	"github.com/jumboframes/armorigo/log"
	"github.com/vishvananda/netlink"
)

func (client *Client) setIPSet() error {
	client.finiIPSet("destroy ipset before init")
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
	return nil
}

func (client *Client) addIPSetIPPort(ip net.IP, port uint16) error {
	err := netlink.IpsetAdd(ConduitIPSetIPPort, &netlink.IPSetEntry{
		IP:   ip,
		Port: &port,
	})
	if err != nil {
		log.Errorf("client add ipset ip: %s, port: %d err: %s", ip, port, err)
	}
	return err
}

func (client *Client) addIPSetPort(port uint16) error {
	err := netlink.IpsetAdd(ConduitIPSetPort, &netlink.IPSetEntry{
		Port: &port,
	})
	if err != nil {
		log.Errorf("client add ipset port: %d err: %s", port, err)
	}
	return err
}

func (client *Client) delIPSetIPPort(ip net.IP, port uint16) error {
	err := netlink.IpsetDel(ConduitIPSetIPPort, &netlink.IPSetEntry{
		IP:   ip,
		Port: &port,
	})
	if err != nil {
		log.Errorf("client del ipset ip: %s, port: %d err: %s", ip, port, err)
	}
	return err
}

func (client *Client) delIPSetPort(port uint16) error {
	err := netlink.IpsetDel(ConduitIPSetPort, &netlink.IPSetEntry{
		Port: &port,
	})
	if err != nil {
		log.Errorf("client del ipset port: %d err: %s", port, err)
	}
	return err
}

func (client *Client) finiIPSet(prefix string) error {
	err := netlink.IpsetFlush(ConduitIPSetIPPort)
	if err != nil {
		log.Warnf("%s, flush ipset: %s err: %s", prefix, ConduitIPSetPort, err)
	}
	err = netlink.IpsetFlush(ConduitIPSetPort)
	if err != nil {
		log.Warnf("%s, flush ipset: %s err: %s", prefix, ConduitIPSetPort, err)
	}

	err = netlink.IpsetDestroy(ConduitIPSetPort)
	if err != nil {
		log.Warnf("%s, destroy ipset: %s err: %s", prefix, ConduitIPSetPort, err)
	}
	err = netlink.IpsetDestroy(ConduitIPSetIPPort)
	if err != nil {
		log.Warnf("%s, destroy ipset: %s, err: %s", prefix, ConduitIPSetIPPort, err)
	}
	return nil
}
