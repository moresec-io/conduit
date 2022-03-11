/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Austin Zhai
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

	"github.com/jumboframes/conduit"
	"github.com/jumboframes/conduit/pkg/log"
	nfw "github.com/jumboframes/conduit/pkg/nf_wrapper"
	"github.com/jumboframes/conduit/pkg/tproxy"
)

const (
	MsProxyChain = "MS_PROXY"
)

type Client struct {
	conf  *conduit.Config
	tp    *tproxy.TProxy
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

	err = client.setTables()
	return err
}

func (client *Client) setTables() error {
	client.finiTables()
	err := client.initTables()
	if err != nil {
		return err
	}
	go func() {
		tick := time.NewTicker(time.Duration(client.conf.Client.Proxy.CheckTime) * time.Second)
		for {
			select {
			case <-tick.C:
				err = client.initTables()
				if err != nil {
					log.Errorf("Client::setTables | init tables err: %s", err)
				}
			case <-client.quit:
				return
			}
		}
	}()
	return nil
}

func (client *Client) initTables() error {
	// check chain exists
	infoO, infoE, err := nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainNew),
		nfw.OptionIptablesChain(MsProxyChain),
	)
	if err != nil && !IsErrChainExists(infoE) {
		log.Errorf("Client::SetTables | new chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
		return err
	}
	// check chain at nat-output
	infoO, infoE, err = nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainCheck),
		nfw.OptionIptablesChain(nfw.IptablesChainOutput),
		nfw.OptionIptablesJump(MsProxyChain),
	)
	if err != nil && !IsErrChainNoMatch(infoE) {
		log.Errorf("Client::SetTables | check output chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
		return err
	}
	if IsErrChainNoMatch(infoE) {
		infoO, infoE, err = nfw.IptablesRun(
			nfw.OptionIptablesWait(),
			nfw.OptionIptablesTable(nfw.IptablesTableNat),
			nfw.OptionIptablesChainOperate(nfw.IptablesChainI),
			nfw.OptionIptablesChain(nfw.IptablesChainOutput),
			nfw.OptionIptablesJump(MsProxyChain),
		)
		if err != nil {
			log.Errorf("Client::SetTables | add output chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
			return err
		}
	}

	for _, transfer := range client.conf.Client.Proxy.Transfers {
		transferIpPort := strings.Split(transfer.Dst, ":")
		ip := transferIpPort[0]
		port, err := strconv.Atoi(transferIpPort[1])
		if err != nil {
			continue
		}
		if ip == "" {
			// only port
			infoO, infoE, err := nfw.IptablesRun(
				nfw.OptionIptablesWait(),
				nfw.OptionIptablesTable(nfw.IptablesTableNat),
				nfw.OptionIptablesChainOperate(nfw.IptablesChainCheck),
				nfw.OptionIptablesChain(MsProxyChain),
				nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
				nfw.OptionIptablesIPv4DstPort(uint32(port)),
				nfw.OptionIptablesJump(nfw.IptablesTargetRedirect),
				nfw.OptionIptablesJumpSubOptions("--to-ports", strconv.Itoa(client.port)),
			)
			if err != nil && !IsErrChainNoMatch(infoE) {
				if IsErrChainExists(infoE) {
					continue
				}
				log.Errorf("Client::SetTables | check chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
				return err
			}

			infoO, infoE, err = nfw.IptablesRun(
				nfw.OptionIptablesWait(),
				nfw.OptionIptablesTable(nfw.IptablesTableNat),
				nfw.OptionIptablesChainOperate(nfw.IptablesChainAdd),
				nfw.OptionIptablesChain(MsProxyChain),
				nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
				nfw.OptionIptablesIPv4DstPort(uint32(port)),
				nfw.OptionIptablesJump(nfw.IptablesTargetRedirect),
				nfw.OptionIptablesJumpSubOptions("--to-ports", strconv.Itoa(client.port)),
			)
			if err != nil {
				if IsErrChainExists(infoE) {
					continue
				}
				log.Errorf("Client::SetTables | add on chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
				return err
			}

		} else {
			// both ip and port
			infoO, infoE, err := nfw.IptablesRun(
				nfw.OptionIptablesWait(),
				nfw.OptionIptablesTable(nfw.IptablesTableNat),
				nfw.OptionIptablesChainOperate(nfw.IptablesChainCheck),
				nfw.OptionIptablesChain(MsProxyChain),
				nfw.OptionIptablesIPv4DstIp(ip),
				nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
				nfw.OptionIptablesIPv4DstPort(uint32(port)),
				nfw.OptionIptablesJump(nfw.IptablesTargetRedirect),
				nfw.OptionIptablesJumpSubOptions("--to-ports", strconv.Itoa(client.port)),
			)
			if err != nil && !IsErrChainNoMatch(infoE) {
				if IsErrChainExists(infoE) {
					continue
				}
				log.Errorf("Client::SetTables | check chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
				return err
			}

			infoO, infoE, err = nfw.IptablesRun(
				nfw.OptionIptablesWait(),
				nfw.OptionIptablesTable(nfw.IptablesTableNat),
				nfw.OptionIptablesChainOperate(nfw.IptablesChainAdd),
				nfw.OptionIptablesChain(MsProxyChain),
				nfw.OptionIptablesIPv4DstIp(ip),
				nfw.OptionIptablesIPv4Proto(nfw.IptablesIPv4Tcp),
				nfw.OptionIptablesIPv4DstPort(uint32(port)),
				nfw.OptionIptablesJump(nfw.IptablesTargetRedirect),
				nfw.OptionIptablesJumpSubOptions("--to-ports", strconv.Itoa(client.port)),
			)
			if err != nil {
				if IsErrChainExists(infoE) {
					continue
				}
				log.Errorf("Client::SetTables | add on chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
				return err
			}
		}
	}
	return nil
}

func (client *Client) finiTables() {
	infoO, infoE, err := nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainFlush),
		nfw.OptionIptablesChain(MsProxyChain),
	)
	if err != nil {
		log.Errorf("Client::SetTables | flush chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
	}

	infoO, infoE, err = nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainDel),
		nfw.OptionIptablesChain(nfw.IptablesChainOutput),
		nfw.OptionIptablesJump(MsProxyChain),
	)
	if err != nil {
		log.Errorf("Client::SetTables | del output chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
	}

	infoO, infoE, err = nfw.IptablesRun(
		nfw.OptionIptablesWait(),
		nfw.OptionIptablesTable(nfw.IptablesTableNat),
		nfw.OptionIptablesChainOperate(nfw.IptablesChainX),
		nfw.OptionIptablesChain(MsProxyChain),
	)
	if err != nil {
		log.Errorf("Client::SetTables | del chain err: %s, infoO: %s, infoE: %s", err, infoO, infoE)
	}
}

func (client *Client) proxy() error {
	tp, err := tproxy.NewTProxy(context.TODO(), client.conf.Client.Proxy.Listen,
		tproxy.OptionTProxyPostAccept(client.tproxyPostAccept),
		tproxy.OptionTProxyPreDial(client.tproxyPreDial),
		tproxy.OptionTProxyPostDial(client.tproxyPostDial),
		tproxy.OptionTProxyPreWrite(client.tproxyPreWrite),
		tproxy.OptionTProxyDial(client.tproxyDial))
	if err != nil {
		return err
	}
	go tp.Listen()
	client.tp = tp
	return nil
}

func (client *Client) Close() {
	client.tp.Close()
	close(client.quit)
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

func (client *Client) tproxyPreDial(pipe *tproxy.Pipe, custom interface{}) error {
	// TODO
	return nil
}

func (client *Client) tproxyDial(pipe *tproxy.Pipe, custom interface{}) (net.Conn, error) {
	ctx := custom.(*ctx)
	switch client.conf.Client.Proxy.Mode {
	case ProxyModeRaw:
		return client.rawDial(pipe, ctx)
	case ProxyModeTls:
		return client.tlsDial(pipe, ctx)
	case ProxyModeMTls:
		return client.mtlsDial(pipe, ctx)
	default:
		return nil, errors.New("unsupported proxy mode")
	}
}

func (client *Client) rawDial(pipe *tproxy.Pipe, ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	log.Debugf("Client::rawDial | src: %s, dst: %s, proxy: %s",
		pipe.Src.String(), pipe.OriginalDst.String(), ctx.proxy)
	conn, err := net.DialTimeout("tcp4",
		ctx.proxy,
		time.Duration(timeout)*time.Second)
	if err != nil {
		log.Errorf("Client::rawDial | src: %s, dst: %s, proxy: %s, err: %s",
			pipe.Src.String(), pipe.OriginalDst.String(), ctx.proxy, err)
		return nil, err
	}
	return conn, nil
}

func (client *Client) tlsDial(pipe *tproxy.Pipe, ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	dialer := net.Dialer{Timeout: time.Duration(timeout) * time.Second}
	log.Debugf("Client::tlsDial | src: %s, dst: %s, proxy: %s",
		pipe.Src.String(), pipe.OriginalDst.String(), ctx.proxy)
	conn, err := tls.DialWithDialer(&dialer, "tcp4",
		ctx.proxy,
		&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Errorf("Client::tlsDial | src: %s, dst: %s, proxy: %s, err: %s",
			pipe.Src.String(), pipe.OriginalDst.String(), ctx.proxy, err)
		return nil, err
	}
	return conn, nil
}

func (client *Client) mtlsDial(pipe *tproxy.Pipe, ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	dialer := net.Dialer{Timeout: time.Duration(timeout) * time.Second}
	log.Debugf("Client::mtlsDial | src: %s, dst: %s, proxy: %s",
		pipe.Src.String(), pipe.OriginalDst.String(), ctx.proxy)
	conn, err := tls.DialWithDialer(&dialer, "tcp4",
		ctx.proxy,
		&tls.Config{InsecureSkipVerify: true, Certificates: client.certs})
	if err != nil {
		log.Errorf("Client::mtlsDial | src: %s, dst: %s, proxy: %s, err: %s",
			pipe.Src.String(), pipe.OriginalDst.String(), ctx.proxy, err)
		return nil, err
	}
	return conn, nil
}

func (client *Client) tproxyPostDial(pipe *tproxy.Pipe, custom interface{}) error {
	// TODO
	return nil
}

func (client *Client) tproxyPreWrite(writer io.Writer, pipe *tproxy.Pipe, custom interface{}) error {
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
