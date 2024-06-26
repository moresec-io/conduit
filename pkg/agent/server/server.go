/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/jumboframes/armorigo/rproxy"
	"github.com/moresec-io/conduit/pkg/agent/config"
	"github.com/moresec-io/conduit/pkg/agent/proto"
	"github.com/moresec-io/conduit/pkg/agent/sys"
)

type Server struct {
	conf *config.Config
	rp   *rproxy.RProxy
	// ca & certs
	caPool *x509.CertPool
	certs  []tls.Certificate

	// listener
	listener net.Listener
}

func NewServer(conf *config.Config) (*Server, error) {
	var err error
	server := &Server{conf: conf}
	switch conf.Server.Proxy.Mode {
	case proto.ProxyModeRaw:
		server.listener, err = net.Listen("tcp4", conf.Server.Proxy.Listen)
		if err != nil {
			return nil, err
		}
	case proto.ProxyModeTls:
		cert, err := tls.LoadX509KeyPair(conf.Server.Cert.CertFile,
			conf.Server.Cert.KeyFile)
		if err != nil {
			return nil, err
		}
		server.listener, err = tls.Listen("tcp4", conf.Server.Proxy.Listen,
			&tls.Config{
				Certificates: []tls.Certificate{cert},
			})
		if err != nil {
			return nil, err
		}
	case proto.ProxyModeMTls:
		ca, err := os.ReadFile(conf.Server.Cert.CaFile)
		if err != nil {
			return nil, err
		}
		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM(ca)
		cert, err := tls.LoadX509KeyPair(conf.Server.Cert.CertFile,
			conf.Server.Cert.KeyFile)
		if err != nil {
			return nil, err
		}
		server.listener, err = tls.Listen("tcp4", conf.Server.Proxy.Listen,
			&tls.Config{
				ClientCAs:    caPool,
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{cert},
			})
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported proxy mode")
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

func (server *Server) replaceDstfunc(conn net.Conn) (net.Addr, net.Conn, error) {
	bs := make([]byte, 4)
	_, err := io.ReadFull(conn, bs)
	if err != nil {
		conn.Close()
		log.Errorf("Server::proxy | read size err: %s", err)
		return nil, nil, err
	}
	data := make([]byte, binary.LittleEndian.Uint32(bs))
	_, err = io.ReadFull(conn, data)
	if err != nil {
		conn.Close()
		log.Errorf("Server::proxy | read meta err: %s", err)
		return nil, nil, err
	}
	proto := &proto.ConduitProto{}
	err = json.Unmarshal(data, proto)
	if err != nil {
		conn.Close()
		log.Errorf("Server::proxy | json unmarshal err: %s", err)
		return nil, nil, err
	}
	log.Debugf("Server::proxy | accept src: %s, dst: %s, to: %s",
		conn.RemoteAddr().String(), conn.LocalAddr().String(), proto.Dst)
	tcpAddr, err := net.ResolveTCPAddr("tcp4", proto.Dst)
	if err != nil {
		conn.Close()
		log.Errorf("Server::proxy | net resolve err: %s", err)
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
