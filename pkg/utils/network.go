package utils

import (
	"net"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

type Network struct {
	Gateway      net.IPNet
	GatewayIndex int
	SubIPs       []net.IP
}

type IPs []net.IP

func ListNetworks() error {
	ips := IPs{}
	networkMap := map[int]*Network{}

	pids, err := ListDifferentNetNamespacePids()
	if err != nil {
		return err
	}
	for _, pid := range pids {
		nshandler, err := netns.GetFromPid(pid)
		if err != nil {
			continue
		}
		handle, err := netlink.NewHandleAt(nshandler)
		if err != nil {
			return err
		}
		links, err := handle.LinkList()
		if err != nil {
			return err
		}
		for _, link := range links {
			if _, isBridge := link.(*netlink.Bridge); isBridge {
				if pid == 1 {
					// default namespace, add to network
				}
			} else if _, isDevice := link.(*netlink.Device); isDevice {
				masterIndex := link.Attrs().MasterIndex
				if masterIndex == 0 {
					// standalone, no master
				} else {
					link, err := netlink.LinkByIndex(masterIndex)
					if err != nil {
						continue
					}
					if _, isBridge := link.(*netlink.Bridge); isBridge {
					}
				}
			}
		}
	}
	return nil
}
