/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"

	"github.com/moresec-io/conduit"
	"github.com/moresec-io/conduit/pkg/log"
	"github.com/moresec-io/conduit/pkg/mtls"
	"github.com/moresec-io/conduit/pkg/openssl"
	"github.com/moresec-io/conduit/pkg/stream"
)

type Server struct {
	conf *conduit.Config

	// ca & certs
	caPool *x509.CertPool
	certs  []tls.Certificate

	// listener
	listener net.Listener
}

func NewServer(conf *conduit.Config) (*Server, error) {
	var err error
	server := &Server{conf: conf}
	switch conf.Server.Proxy.Mode {
	case ProxyModeRaw:
		server.listener, err = net.Listen("tcp4", conf.Server.Proxy.Listen)
		if err != nil {
			return nil, err
		}
	case ProxyModeTls:
		cert, err := mtls.LoadX509KeyPair(conf.Server.Cert.CertFile, conf.Server.Cert.KeyFile, "")
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
	case ProxyModeMTls:
		ca, err := ioutil.ReadFile(conf.Server.Cert.CaFile)
		if err != nil {
			return nil, err
		}
		caDecrypt, err := openssl.Decrypt(openssl.TpAes, string(ca), "")
		if err != nil {
			return nil, err
		}
		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM([]byte(caDecrypt))
		cert, err := mtls.LoadX509KeyPair(conf.Server.Cert.CertFile, conf.Server.Cert.KeyFile, "")
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
	for {
		conn, err := server.listener.Accept()
		if err != nil {
			return err
		}

		proxy := func(conn net.Conn) {
			bs := make([]byte, 4)
			_, err := io.ReadFull(conn, bs)
			if err != nil {
				conn.Close()
				log.Errorf("Server::proxy | read size err: %s", err)
				return
			}
			data := make([]byte, binary.LittleEndian.Uint32(bs))
			_, err = io.ReadFull(conn, data)
			if err != nil {
				conn.Close()
				log.Errorf("Server::proxy | read meta err: %s", err)
				return
			}
			proto := &MSProxyProto{}
			err = json.Unmarshal(data, proto)
			if err != nil {
				conn.Close()
				log.Errorf("Server::proxy | json unmarshal err: %s", err)
				return
			}
			log.Debugf("Server::proxy | accept src: %s, dst: %s, to: %s",
				conn.RemoteAddr().String(), conn.LocalAddr().String(), proto.Dst)
			tcpAddr, err := net.ResolveTCPAddr("tcp4", proto.Dst)
			if err != nil {
				conn.Close()
				log.Errorf("Server::proxy | net resolve err: %s", err)
				return
			}
			p, err := stream.NewTCPProxy(conn, tcpAddr, false, control)
			if err != nil {
				conn.Close()
				log.Errorf("Server::proxy | new tcp proxy err: %s", err)
				return
			}
			p.Proxy()
		}

		go proxy(conn)
	}
}

func (server *Server) Close() {
	server.listener.Close()
}
