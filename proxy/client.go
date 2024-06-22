/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package proxy

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

	"github.com/moresec-io/conduit"
	"github.com/moresec-io/conduit/pkg/log"
	"github.com/moresec-io/conduit/pkg/mtls"
	"github.com/moresec-io/conduit/pkg/pproxy"
	"github.com/moresec-io/conduit/pkg/tproxy"
)

const (
	MsProxyChain = "MS_PROXY"
)

type Client struct {
	conf  *conduit.Config
	tp    *tproxy.TProxy
	pps   []*pproxy.PProxy
	certs []tls.Certificate
	quit  chan struct{}
	// listen port
	port int
}

func NewClient(conf *conduit.Config) (*Client, error) {
	client := &Client{
		conf: conf,
		quit: make(chan struct{}),
	}
	if conf.Client.Proxy.Mode == ProxyModeMTls {
		cert, err := mtls.LoadX509KeyPair(conf.Client.Cert.CertFile, conf.Client.Cert.KeyFile, "")
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
	switch client.conf.Client.Proxy.LocalMode {
	default:
		err := client.tproxy()
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
	case "port":
		err := client.pproxy()
		if err != nil {
			return err
		}
	}
	return nil
}

// port proxy
func (client *Client) pproxy() error {
	client.pps = []*pproxy.PProxy{}
	for _, transfer := range client.conf.Client.Proxy.Transfers {
		tp, err := pproxy.NewPProxy(context.TODO(), transfer.Dst,
			pproxy.OptionPProxyPostAccept(client.proxyPostAccept),
			pproxy.OptionPProxyPreDial(client.proxyPreDial),
			pproxy.OptionPProxyPostDial(client.proxyPostDial),
			pproxy.OptionPProxyPreWrite(client.proxyPreWrite),
			pproxy.OptionPProxyDial(client.proxyDial))
		if err != nil {
			return err
		}
		go tp.Listen()
		client.pps = append(client.pps, tp)
	}
	return nil
}

func (client *Client) proxyPostAccept(src, dst net.Addr) (interface{}, error) {
	log.Debugf("Client::pproxyPostAccept | src: %s, dst: %s", src.String(), dst.String())

	ipPort := strings.Split(src.String(), ":")
	srcIp := ipPort[0]
	srcPort, err := strconv.Atoi(ipPort[1])
	if err != nil {
		log.Errorf("Client::proxyPostAccept | src port to int err: %s", err)
		return nil, err
	}
	ipPort = strings.Split(dst.String(), ":")
	dstIp := ipPort[0]
	dstPort, err := strconv.Atoi(ipPort[1])
	if err != nil {
		log.Errorf("Client::proxyPostAccept | dst port to int err: %s", err)
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
	log.Info(ctx)
	return ctx, nil
}

// transparent proxy
func (client *Client) tproxy() error {
	tp, err := tproxy.NewTProxy(context.TODO(), client.conf.Client.Proxy.Listen,
		tproxy.OptionTProxyPostAccept(client.proxyPostAccept),
		tproxy.OptionTProxyPreDial(client.proxyPreDial),
		tproxy.OptionTProxyPostDial(client.proxyPostDial),
		tproxy.OptionTProxyPreWrite(client.proxyPreWrite),
		tproxy.OptionTProxyDial(client.proxyDial))
	if err != nil {
		return err
	}
	go tp.Listen()
	client.tp = tp
	return nil
}

func (client *Client) Close() {
	if client.tp != nil {
		client.tp.Close()
	}
	if client.pps != nil {
		for _, pp := range client.pps {
			pp.Close()
		}
	}
	close(client.quit)
	client.finiTables()
}

type ctx struct {
	srcIp   string //源ip
	srcPort int    //源port
	dstIp   string //原始目的ip
	dstPort int    //原始目的port
	proxy   string //代理
	dst     string // 代理后地址
}

func (client *Client) rawDial(ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	log.Debugf("Client::rawDial | src: %s:%d, dst: %s:%d, proxy: %s",
		ctx.srcIp, ctx.srcPort, ctx.dstIp, ctx.dstPort, ctx.proxy)
	conn, err := net.DialTimeout("tcp4",
		ctx.proxy,
		time.Duration(timeout)*time.Second)
	if err != nil {
		log.Errorf("Client::rawDial | src: %s, dst: %s, proxy: %s, err: %s",
			ctx.srcIp, ctx.srcPort, ctx.dstIp, ctx.dstPort, ctx.proxy, err)
		return nil, err
	}
	return conn, nil
}

func (client *Client) tlsDial(ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	dialer := net.Dialer{
		Timeout: time.Duration(timeout) * time.Second,
		Control: control,
	}
	log.Debugf("Client::tlsDial | src: %s:%d, dst: %s:%d, proxy: %s",
		ctx.srcIp, ctx.srcPort, ctx.dstIp, ctx.dstPort, ctx.proxy)
	conn, err := tls.DialWithDialer(&dialer, "tcp4",
		ctx.proxy,
		&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Errorf("Client::tlsDial | src: %s, dst: %s, proxy: %s, err: %s",
			ctx.srcIp, ctx.srcPort, ctx.dstIp, ctx.dstPort, ctx.proxy, err)
		return nil, err
	}
	return conn, nil
}

func (client *Client) mtlsDial(ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	dialer := net.Dialer{
		Timeout: time.Duration(timeout) * time.Second,
		Control: control,
	}
	log.Debugf("Client::mtlsDial | src: %s:%d, dst: %s:%d, proxy: %s",
		ctx.srcIp, ctx.srcPort, ctx.dstIp, ctx.dstPort, ctx.proxy)
	conn, err := tls.DialWithDialer(&dialer, "tcp4",
		ctx.proxy,
		&tls.Config{InsecureSkipVerify: true, Certificates: client.certs})
	if err != nil {
		log.Errorf("Client::mtlsDial | src: %s:%d, dst: %s:%d, proxy: %s, err: %s",
			ctx.srcIp, ctx.srcPort, ctx.dstIp, ctx.dstPort, ctx.proxy, err)
		return nil, err
	}
	return conn, nil
}

func (client *Client) proxyPreDial(custom interface{}) error {
	// TODO
	return nil
}

// func (client *Client) proxyDial(pipe *tproxy.Pipe, custom interface{}) (net.Conn, error) {
func (client *Client) proxyDial(dst net.Addr, custom interface{}) (net.Conn, error) {
	ctx := custom.(*ctx)
	switch client.conf.Client.Proxy.Mode {
	case ProxyModeRaw:
		return client.rawDial(ctx)
	case ProxyModeTls:
		return client.tlsDial(ctx)
	case ProxyModeMTls:
		return client.mtlsDial(ctx)
	default:
		return nil, errors.New("unsupported proxy mode")
	}
}

func (client *Client) proxyPostDial(custom interface{}) error {
	// TODO
	return nil
}

func (client *Client) proxyPreWrite(writer io.Writer, custom interface{}) error {
	ctx := custom.(*ctx)
	proto := &MSProxyProto{
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
