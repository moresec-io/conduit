//go:build linux && amd64
// +build linux,amd64

/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */

package tproxy

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/moresec-io/conduit/pkg/log"
)

func rawSyscallDial(pipe *Pipe, custom interface{}) (net.Conn, error) {
	var rightConn net.Conn
	var err error

	fd, err := newSocket()
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)

	sockAddr, err := netAddr2Sockaddr(pipe.OriginalDst)
	if err != nil {
		return nil, err
	}

	if err = connect(fd,
		sockAddr,
		time.Now().Add(3*time.Second)); err != nil {

		log.Errorf("connect err: %s", err)
		return nil, err
	}

	f := os.NewFile(uintptr(fd), "port."+strconv.Itoa(os.Getpid()))
	rightConn, err = net.FileConn(f)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return rightConn, nil
}

func newSocket() (fd int, err error) {
	syscall.ForkLock.RLock()
	fd, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if err == nil {
		syscall.CloseOnExec(fd)
	}
	syscall.ForkLock.RUnlock()

	if err != nil {
		return -1, err
	}

	if err = syscall.SetNonblock(fd, true); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, 2515); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	return fd, err
}

func netAddr2Sockaddr(addr net.Addr) (syscall.Sockaddr, error) {
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return nil, errors.New("not tcp address")
	}
	ipv4 := tcpAddr.IP.To4()
	if ipv4 == nil {
		return nil, errors.New("invalid ipv4 address")
	}
	var buf [4]byte
	copy(buf[:], ipv4)
	sockAddr := &syscall.SockaddrInet4{Addr: buf, Port: tcpAddr.Port}
	return sockAddr, nil
}

// this is close to the connect() function inside stdlib/net
func connect(fd int, ra syscall.Sockaddr, deadline time.Time) error {
	switch err := syscall.Connect(fd, ra); err {
	case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
	case nil, syscall.EISCONN:
		if !deadline.IsZero() && deadline.Before(time.Now()) {
			return errors.New("io timeout")
		}
		return nil
	default:
		return err
	}

	var err error
	var to syscall.Timeval
	var toptr *syscall.Timeval
	var pw syscall.FdSet
	FD_SET(uintptr(fd), &pw)
	for {
		// wait until the fd is ready to read or write.
		if !deadline.IsZero() {
			to = syscall.NsecToTimeval(deadline.Sub(time.Now()).Nanoseconds())
			toptr = &to
		}

		// wait until the fd is ready to write. we can't use:
		//   if err := fd.pd.WaitWrite(); err != nil {
		//   	 return err
		//   }
		// so we use select instead.
		if _, err = Select(fd+1, nil, &pw, nil, toptr); err != nil {
			fmt.Println(err)
			return err
		}

		var nerr int
		nerr, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
		if err != nil {
			return err
		}
		switch err = syscall.Errno(nerr); err {
		case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
			continue
		case syscall.Errno(0), syscall.EISCONN:
			if !deadline.IsZero() && deadline.Before(time.Now()) {
				return errors.New("io timeout")
			}
			return nil
		default:
			return err
		}
	}
}

func FD_SET(fd uintptr, p *syscall.FdSet) {
	n, k := fd/32, fd%32
	p.Bits[n] |= 1 << uint32(k)
}

func Select(nfd int, r *syscall.FdSet, w *syscall.FdSet, e *syscall.FdSet, timeout *syscall.Timeval) (int, error) {
	return syscall.Select(nfd, r, w, e, timeout)
}
