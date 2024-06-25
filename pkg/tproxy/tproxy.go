/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package tproxy

import (
	"fmt"
	"net"
	"syscall"

	"github.com/jumboframes/armorigo/log"
)

const (
	SO_ORIGINAL_DST = 80
)

// 注意没有拿到originalDst就会关闭连接
func AcquireOriginalDst(conn net.Conn) (net.Addr, net.Conn, error) {
	var err error
	//需要从conn中找到原始dst addr

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		if err = conn.Close(); err != nil {
			log.Errorf("close non tcp conn err: %s", err)
		}
		return nil, nil, err
	}
	tcpFile, err := tcpConn.File()
	if err != nil {
		log.Errorf("get tcp file from tcp conn err: %s", err)
		if err = tcpConn.Close(); err != nil {
			log.Errorf("close tcp conn err: %s", err)
		}
		return nil, nil, err
	}
	if err = tcpConn.Close(); err != nil {
		log.Errorf("close tcp conn err: %s", err)
		if err = tcpFile.Close(); err != nil {
			log.Errorf("close tcp file err: %s", err)
		}
		return nil, nil, err
	}

	//the real shit
	mreq, err := syscall.GetsockoptIPv6Mreq(
		int(tcpFile.Fd()),
		syscall.IPPROTO_IP,
		SO_ORIGINAL_DST)
	if err != nil {
		log.Errorf("get sock opt ipv6 mreq err: %s", err)
		if err = tcpFile.Close(); err != nil {
			log.Errorf("close tcp file err: %s", err)
		}
		return nil, nil, err
	}

	ipv4 := net.IPv4(
		mreq.Multiaddr[4],
		mreq.Multiaddr[5],
		mreq.Multiaddr[6],
		mreq.Multiaddr[7])
	port := uint16(mreq.Multiaddr[2])<<8 + uint16(mreq.Multiaddr[3])
	originalDst, _ := net.ResolveTCPAddr("tcp4",
		fmt.Sprintf("%s:%d", ipv4.String(), port))

	//restore conn
	fileConn, err := net.FileConn(tcpFile)
	if err != nil {
		log.Errorf("get file conn from tcp file err: %s", err)
		if err = tcpFile.Close(); err != nil {
			log.Errorf("close tcp file err: %s", err)
		}
		return nil, nil, err
	}
	if err = tcpFile.Close(); err != nil {
		log.Errorf("close tcp file err: %s", err)
	}

	leftConn := fileConn.(*net.TCPConn)
	return originalDst, leftConn, nil
}
