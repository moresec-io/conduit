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
	"strconv"

	"github.com/jumboframes/conduit"
	"github.com/jumboframes/conduit/pkg/log"
	"github.com/jumboframes/conduit/pkg/stream"
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
	case ProxyModeMTls:
		ca, err := ioutil.ReadFile(conf.Server.Cert.CaFile)
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

func (server *Server) Proxy() error {
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
				log.Errorf("Server::Proxy | read size err: %s", err)
				return
			}
			data := make([]byte, binary.LittleEndian.Uint32(bs))
			_, err = io.ReadFull(conn, data)
			if err != nil {
				conn.Close()
				log.Errorf("Server::Proxy | read meta err: %s", err)
				return
			}
			proto := &MSProxyProto{}
			err = json.Unmarshal(data, proto)
			if err != nil {
				conn.Close()
				log.Errorf("Server::Proxy | json unmarshal err: %s", err)
				return
			}
			addr := proto.DstIpOrigin + ":" + strconv.Itoa(proto.DstPortOrigin)
			tcpAddr, err := net.ResolveTCPAddr("tcp4", addr)
			if err != nil {
				conn.Close()
				log.Errorf("Server::Proxy | net resolve err: %s", err)
				return
			}
			p, err := stream.NewTCPProxy(conn, tcpAddr, false)
			if err != nil {
				conn.Close()
				log.Errorf("Server::Proxy | new tcp proxy err: %s", err)
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
