/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package client

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/jumboframes/armorigo/rproxy"
	"github.com/moresec-io/conduit/pkg/agent/config"
	"github.com/moresec-io/conduit/pkg/agent/proto"
	"github.com/moresec-io/conduit/pkg/agent/sys"
	"github.com/moresec-io/conduit/pkg/tproxy"
)

const (
	ConduitChain       = "CONDUIT"
	ConduitIPSetPort   = "CONDUIT_PORT"
	ConduitIPSetIPPort = "CONDUIT_IPPORT"
)

type Client struct {
	conf  *config.Config
	rp    *rproxy.RProxy
	certs []tls.Certificate
	quit  chan struct{}
	// listen port
	port int
}

func NewClient(conf *config.Config) (*Client, error) {
	client := &Client{
		conf: conf,
		quit: make(chan struct{}),
	}
	if conf.Client.Proxy.Mode == proto.ProxyModeMTls {
		cert, err := tls.LoadX509KeyPair(conf.Client.Cert.CertFile,
			conf.Client.Cert.KeyFile)
		if err != nil {
			return nil, err
		}
		client.certs = []tls.Certificate{cert}
	}
	ipPort := strings.Split(conf.Client.Proxy.Listen, ":")
	if len(ipPort) != 2 {
		return nil, errors.New("illegal client listen addr")
	}
	port, err := strconv.Atoi(ipPort[1])
	if err != nil {
		return nil, err
	}
	client.port = port
	return client, nil
}

func (client *Client) Work() error {
	err := client.proxy()
	if err != nil {
		return err
	}
	err = client.setIPSet()
	if err != nil {
		return err
	}

	err = client.setTables()
	if err != nil {
		return err
	}

	err = client.setProc()
	if err != nil {
		return err
	}

	err = client.setStaticMaps()
	if err != nil {
		return err
	}
	return nil
}

func (client *Client) setStaticMaps() error {
	for _, transfer := range client.conf.Client.Proxy.Transfers {
		transferIpPort := strings.Split(transfer.Dst, ":")
		ip := transferIpPort[0]
		port, err := strconv.Atoi(transferIpPort[1])
		if err != nil {
			return err
		}
		if ip == "" {
			err = client.addIPSetPort(uint16(port))
			if err != nil {
				return err
			}
		} else {
			err = client.addIPSetIPPort(net.ParseIP(ip), uint16(port))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (client *Client) proxy() error {
	listener, err := net.Listen("tcp4", client.conf.Client.Proxy.Listen)
	if err != nil {
		return err
	}
	rp, err := rproxy.NewRProxy(listener,
		rproxy.OptionRProxyPostAccept(client.tproxyPostAccept),
		rproxy.OptionRProxyPreDial(client.tproxyPreDial),
		rproxy.OptionRProxyPostDial(client.tproxyPostDial),
		rproxy.OptionRProxyPreWrite(client.tproxyPreWrite),
		rproxy.OptionRProxyReplaceDst(tproxy.AcquireOriginalDst),
		rproxy.OptionRProxyDial(client.tproxyDial))
	if err != nil {
		return err
	}
	go rp.Proxy(context.TODO())
	client.rp = rp
	return nil
}

func (client *Client) Close() {
	client.rp.Close()
	close(client.quit)
	client.finiTables("client fini tables")
	client.finiIPSet("client fini ipset")
}

type ctx struct {
	srcIp   string //源ip
	srcPort int    //源port
	dstIp   string //原始目的ip
	dstPort int    //原始目的port
	proxy   string //代理
	dst     string // 代理后地址
}

func (client *Client) tproxyPostAccept(src, dst net.Addr) (interface{}, error) {
	log.Debugf("Client::tproxyPostAccept | src: %s, dst: %s", src.String(), dst.String())

	ipPort := strings.Split(src.String(), ":")
	srcIp := ipPort[0]
	srcPort, err := strconv.Atoi(ipPort[1])
	if err != nil {
		log.Errorf("Client::tproxyPostAccept | src port to int err: %s", err)
		return nil, err
	}
	ipPort = strings.Split(dst.String(), ":")
	dstIp := ipPort[0]
	dstPort, err := strconv.Atoi(ipPort[1])
	if err != nil {
		log.Errorf("Client::tproxyPostAccept | dst port to int err: %s", err)
		return nil, err
	}

	ctx := &ctx{
		srcIp:   srcIp,
		srcPort: srcPort,
		dstIp:   dstIp,
		dstPort: dstPort,
		proxy:   dstIp + ":" + strconv.Itoa(client.conf.Client.Proxy.ServerPort),
		dst:     dstIp + ":" + strconv.Itoa(dstPort),
	}
	for _, transfer := range client.conf.Client.Proxy.Transfers {
		transferIpPort := strings.Split(transfer.Dst, ":")
		if transferIpPort[0] == "" || transferIpPort[0] == dstIp {
			if transferIpPort[1] == ipPort[1] {
				// match
				if transfer.Proxy != "" {
					ctx.proxy = transfer.Proxy + ":" +
						strconv.Itoa(client.conf.Client.Proxy.ServerPort)
				}
				if transfer.DstTo != "" {
					ctx.dst = transfer.DstTo
				}
				break
			}
		}
	}
	return ctx, nil
}

func (client *Client) tproxyPreDial(custom interface{}) error {
	// TODO
	return nil
}

func (client *Client) tproxyDial(dst net.Addr, custom interface{}) (net.Conn, error) {
	ctx := custom.(*ctx)
	switch client.conf.Client.Proxy.Mode {
	case proto.ProxyModeRaw:
		return client.rawDial(dst, ctx)
	case proto.ProxyModeTls:
		return client.tlsDial(dst, ctx)
	case proto.ProxyModeMTls:
		return client.mtlsDial(dst, ctx)
	default:
		return nil, errors.New("unsupported proxy mode")
	}
}

func (client *Client) rawDial(dst net.Addr, ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	log.Debugf("Client::rawDial | dst: %s, proxy: %s", dst, ctx.proxy)
	conn, err := net.DialTimeout("tcp4",
		ctx.proxy,
		time.Duration(timeout)*time.Second)
	if err != nil {
		log.Errorf("Client::rawDial | dst: %s, proxy: %s, err: %s", dst.String(), ctx.proxy, err)
		return nil, err
	}
	return conn, nil
}

func (client *Client) tlsDial(dst net.Addr, ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	dialer := net.Dialer{
		Timeout: time.Duration(timeout) * time.Second,
		Control: sys.Control,
	}
	log.Debugf("Client::tlsDial | dst: %s, proxy: %s", dst.String(), ctx.proxy)
	conn, err := tls.DialWithDialer(&dialer, "tcp4",
		ctx.proxy,
		&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Errorf("Client::tlsDial | dst: %s, proxy: %s, err: %s", dst.String(), ctx.proxy, err)
		return nil, err
	}
	return conn, nil
}

func (client *Client) mtlsDial(dst net.Addr, ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	dialer := net.Dialer{
		Timeout: time.Duration(timeout) * time.Second,
		Control: sys.Control,
	}
	log.Debugf("Client::mtlsDial | dst: %s, proxy: %s", dst.String(), ctx.proxy)
	conn, err := tls.DialWithDialer(&dialer, "tcp4",
		ctx.proxy,
		&tls.Config{InsecureSkipVerify: true, Certificates: client.certs})
	if err != nil {
		log.Errorf("Client::mtlsDial | dst: %s, proxy: %s, err: %s", dst.String(), ctx.proxy, err)
		return nil, err
	}
	return conn, nil
}

func (client *Client) tproxyPostDial(custom interface{}) error {
	// TODO
	return nil
}

func (client *Client) tproxyPreWrite(writer io.Writer, custom interface{}) error {
	ctx := custom.(*ctx)
	proto := &proto.ConduitProto{
		SrcIp:   ctx.srcIp,
		SrcPort: ctx.srcPort,
		DstIp:   ctx.dstIp,
		DstPort: ctx.dstPort,
		Proxy:   ctx.proxy,
		Dst:     ctx.dst,
	}
	data, err := json.Marshal(proto)
	if err != nil {
		return err
	}
	length := uint32(len(data))
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, length)
	_, err = writer.Write(bs)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	if err != nil {
		return err
	}
	return nil
}
