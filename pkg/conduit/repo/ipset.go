package repo

import (
	"net"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/conduit/errors"
	"github.com/vishvananda/netlink"
)

const (
	// ipset
	ConduitIPSetPort   = "CONDUIT_PORT"
	ConduitIPSetIPPort = "CONDUIT_IPPORT"
	ConduitIPSetIP     = "CONDUIT_IP"
)

// A wrapper
type ipset struct{}

func (ipset *ipset) InitIPSet() error {
	return initIPSet()
}

func (ipset *ipset) AddIPSetIPPort(ip net.IP, port uint16) error {
	return addIPSetIPPort(ip, port)
}

func (ipset *ipset) AddIPSetPort(port uint16) error {
	return addIPSetPort(port)
}

func (ipset *ipset) AddIPSetIP(ip net.IP) error {
	return addIPSetIP(ip)
}

func (ipset *ipset) DelIPSetIPPort(ip net.IP, port uint16) error {
	return delIPSetIPPort(ip, port)
}

func (ipset *ipset) DelIPSetPort(port uint16) error {
	return delIPSetPort(port)
}

func (ipset *ipset) DelIPSetIP(ip net.IP) error {
	return delIPSetIP(ip)
}

func (ipset *ipset) FiniIPSet(level log.Level, prefix string) error {
	return finiIPSet(level, prefix)
}

func initIPSet() error {
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

func addIPSetIPPort(ip net.IP, port uint16) error {
	err := netlink.IpsetAdd(ConduitIPSetIPPort, &netlink.IPSetEntry{
		IP:   ip,
		Port: &port,
	})
	if err != nil {
		log.Errorf("client add ipset ip: %s, port: %d err: %s", ip, port, err)
	}
	return err
}

func addIPSetPort(port uint16) error {
	err := netlink.IpsetAdd(ConduitIPSetPort, &netlink.IPSetEntry{
		Port: &port,
	})
	if err != nil {
		log.Errorf("client add ipset port: %d err: %s", port, err)
	}
	return err
}

func addIPSetIP(ip net.IP) error {
	err := netlink.IpsetAdd(ConduitIPSetIP, &netlink.IPSetEntry{
		IP: ip,
	})
	if err != nil {
		log.Errorf("client add ip: %s err: %s", ip, err)
	}
	return err
}

func delIPSetIPPort(ip net.IP, port uint16) error {
	err := netlink.IpsetDel(ConduitIPSetIPPort, &netlink.IPSetEntry{
		IP:   ip,
		Port: &port,
	})
	if err != nil {
		log.Errorf("client del ipset ip: %s, port: %d err: %s", ip, port, err)
	}
	return err
}

func delIPSetPort(port uint16) error {
	err := netlink.IpsetDel(ConduitIPSetPort, &netlink.IPSetEntry{
		Port: &port,
	})
	if err != nil {
		log.Errorf("client del ipset port: %d err: %s", port, err)
	}
	return err
}

func delIPSetIP(ip net.IP) error {
	err := netlink.IpsetDel(ConduitIPSetIP, &netlink.IPSetEntry{
		IP: ip,
	})
	if err != nil {
		log.Errorf("client del ip: %s err: %s", ip, err)
	}
	return err
}

func finiIPSet(level log.Level, prefix string) error {
	// flush
	err := netlink.IpsetFlush(ConduitIPSetIPPort)
	if err != nil && !errors.IsErrNoSuchFileOrDirectory(err) {
		log.Printf(level, "%s, flush ipset: %s err: %s", prefix, ConduitIPSetPort, err)
	}
	err = netlink.IpsetFlush(ConduitIPSetPort)
	if err != nil && !errors.IsErrNoSuchFileOrDirectory(err) {
		log.Printf(level, "%s, flush ipset: %s err: %s", prefix, ConduitIPSetPort, err)
	}
	err = netlink.IpsetFlush(ConduitIPSetIP)
	if err != nil && !errors.IsErrNoSuchFileOrDirectory(err) {
		log.Printf(level, "%s, flush ipset: %s err: %s", prefix, ConduitIPSetIP, err)
	}

	// destroy
	err = netlink.IpsetDestroy(ConduitIPSetPort)
	if err != nil && !errors.IsErrNoSuchFileOrDirectory(err) {
		log.Printf(level, "%s, destroy ipset: %s err: %s", prefix, ConduitIPSetPort, err)
	}
	err = netlink.IpsetDestroy(ConduitIPSetIPPort)
	if err != nil && !errors.IsErrNoSuchFileOrDirectory(err) {
		log.Printf(level, "%s, destroy ipset: %s err: %s", prefix, ConduitIPSetIPPort, err)
	}
	err = netlink.IpsetDestroy(ConduitIPSetIP)
	if err != nil && !errors.IsErrNoSuchFileOrDirectory(err) {
		log.Printf(level, "%s, destroy ipset: %s err: %s", prefix, ConduitIPSetIP, err)
	}
	return nil
}
