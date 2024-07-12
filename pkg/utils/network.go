package utils

import (
	"net"
	"syscall"
	"unsafe"

	"github.com/vishvananda/netlink"
)

func ListNetworks() ([]net.IPNet, error) {
	ipNets := []net.IPNet{}

	handle, err := netlink.NewHandle()
	if err != nil {
		return nil, err
	}
	links, err := handle.LinkList()
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		if link.Attrs().Name == "lo" {
			continue
		}
		if _, isBridge := link.(*netlink.Bridge); isBridge {
			// we don't handle bridge for now
			continue
		}
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			ipNets = append(ipNets, *addr.IPNet)
		}
	}
	return ipNets, nil
}

func GetSocketMark(fd uintptr) (uint32, error) {
	var mark uint32
	size := unsafe.Sizeof(mark)
	_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, syscall.SOL_SOCKET, syscall.SO_MARK, uintptr(unsafe.Pointer(&mark)), uintptr(unsafe.Pointer(&size)), 0)
	if errno != 0 {
		return 0, errno
	}
	return mark, nil
}
