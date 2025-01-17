/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/jumboframes/armorigo/rproxy"
	"github.com/moresec-io/conduit/pkg/conduit/config"
	"github.com/moresec-io/conduit/pkg/conduit/proto"
	"github.com/moresec-io/conduit/pkg/conduit/syncer"
	"github.com/moresec-io/conduit/pkg/conduit/sys"
	"github.com/moresec-io/conduit/pkg/network"
	gproto "github.com/moresec-io/conduit/pkg/proto"
)

type Server struct {
	conf   *config.Config
	rp     *rproxy.RProxy
	syncer syncer.Syncer

	// listener
	listener net.Listener
}

func NewServer(conf *config.Config, syncer syncer.Syncer) (*Server, error) {
	var (
		err error
		tls *gproto.TLS
	)
	if conf.Manager.Enable {
		ips, err := network.ListIPs()
		if err != nil {
			return nil, err
		}
		// if manager configured, then use cluster tls configuration
		response, err := syncer.ReportServer(&gproto.ReportServerRequest{
			MachineID: conf.MachineID,
			Network:   conf.Server.Network,
			Addr:      conf.Server.Addr,
			IPs:       ips,
		})
		if err != nil {
			return nil, err
		}
		err = syncer.ReportNetworks()
		if err != nil {
			return nil, err
		}
		tls = response.TLS
	}
	server := &Server{
		conf:   conf,
		syncer: syncer,
	}
	if tls != nil {
		server.listener, err = network.ListenDERMTLS(conf.Server.Network, conf.Server.Addr, tls.CA, tls.Cert, tls.Key)
	} else {
		server.listener, err = network.Listen(&conf.Server.Listen)
	}
	if err != nil {
		return nil, err
	}
	return server, nil
}

func (server *Server) Work() error {
	err := server.proxy()
	if err != nil {
		return err
	}
	return nil
}

func (server *Server) proxy() error {
	rp, err := rproxy.NewRProxy(server.listener,
		rproxy.OptionRProxyDial(server.dial),
		rproxy.OptionRProxyReplaceDst(server.replaceDstfunc))
	if err != nil {
		return err
	}
	go rp.Proxy(context.TODO())
	server.rp = rp
	return nil
}

func (server *Server) replaceDstfunc(conn net.Conn, meta ...interface{}) (net.Addr, net.Conn, error) {
	bs := make([]byte, 4)
	_, err := io.ReadFull(conn, bs)
	if err != nil {
		conn.Close()
		log.Errorf("server replace dst func, read size err: %s", err)
		return nil, nil, err
	}
	data := make([]byte, binary.LittleEndian.Uint32(bs))
	_, err = io.ReadFull(conn, data)
	if err != nil {
		conn.Close()
		log.Errorf("server replace dst func, read meta err: %s", err)
		return nil, nil, err
	}
	proto := &proto.ConduitProto{}
	err = json.Unmarshal(data, proto)
	if err != nil {
		conn.Close()
		log.Errorf("server replace dst func, json unmarshal err: %s", err)
		return nil, nil, err
	}
	log.Debugf("server replace dst func, accept src: %s, dst: %s, as: %s",
		conn.RemoteAddr().String(), conn.LocalAddr().String(), proto.DstAs)
	tcpAddr, err := net.ResolveTCPAddr("tcp4", proto.DstAs)
	if err != nil {
		conn.Close()
		log.Errorf("server replace dst func, net resolve err: %s", err)
		return nil, nil, err
	}
	return tcpAddr, conn, nil
}

func (server *Server) dial(dst net.Addr, custom interface{}) (net.Conn, error) {
	timeout := time.Second * 10
	dialer := net.Dialer{
		Timeout: timeout,
		Control: sys.Control,
	}
	return dialer.Dial("tcp", dst.String())
}

func (server *Server) Close() {
	server.listener.Close()
}
