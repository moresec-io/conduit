/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package client

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/jumboframes/armorigo/log"
	"github.com/jumboframes/armorigo/rproxy"
	"github.com/moresec-io/conduit/pkg/conduit/config"
	"github.com/moresec-io/conduit/pkg/conduit/proto"
	gconfig "github.com/moresec-io/conduit/pkg/config"
	"github.com/moresec-io/conduit/pkg/tproxy"
	"github.com/moresec-io/conduit/pkg/utils"
)

const (
	ConduitChain       = "CONDUIT"
	ConduitIPSetPort   = "CONDUIT_PORT"
	ConduitIPSetIPPort = "CONDUIT_IPPORT"
	ConduitIPSetIP     = "CONDUIT_IP"
)

type policy struct {
	dial  *gconfig.Dial
	dstTo string
}

type Client struct {
	conf *config.Config
	rp   *rproxy.RProxy
	quit chan struct{}
	// listen port
	port int

	// static policies
	ipportPolicies map[string]*policy
	portPolicies   map[int]*policy
}

func NewClient(conf *config.Config) (*Client, error) {
	client := &Client{
		conf: conf,
		quit: make(chan struct{}),
	}
	ipPort := strings.Split(conf.Client.Listen, ":")
	if len(ipPort) != 2 {
		return nil, errors.New("illegal client listen addr")
	}
	port, err := strconv.Atoi(ipPort[1])
	if err != nil {
		return nil, err
	}
	client.port = port

	// static policy match
	for _, policy := range conf.Client.Policies {
		ipport := strings.Split(policy.Dst, ":")
		if len(ipport) != 2 {
			// TODO warning
			continue
		}
		ipstr := ipport[0]
		portstr := ipport[1]
		port, err := strconv.Atoi(portstr)
		if err != nil {
			// TODO warning
			continue
		}
	}
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

	err = client.setStaticPolicies()
	if err != nil {
		return err
	}
	return nil
}

func (client *Client) setStaticPolicies() error {
	for _, policy := range client.conf.Client.Policies {
		transferIpPort := strings.Split(policy.Dst, ":")
		ip := transferIpPort[0]
		port, err := strconv.Atoi(transferIpPort[1])
		if err != nil {
			return err
		}
		if ip == "" {
			err = client.AddIPSetPort(uint16(port))
			if err != nil {
				return err
			}
		} else {
			err = client.AddIPSetIPPort(net.ParseIP(ip), uint16(port))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (client *Client) proxy() error {
	listener, err := net.Listen("tcp4", client.conf.Client.Listen)
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
	// connection info
	srcIp   string // source ip
	srcPort int    // source port
	dstIp   string // real dst ip
	dstPort int    // real dst port
	mark    uint32
	// proxy info
	dial  *gconfig.Dial // proxy
	dstTo string        // dst after proxy
}

func (client *Client) tproxyPostAccept(src, dst net.Addr) (interface{}, error) {
	log.Debugf("client tproxy post accept, src: %s, dst: %s", src.String(), dst.String())

	conf := &client.conf.Client

	ipPort := strings.Split(src.String(), ":")
	srcIp := ipPort[0]
	srcPort, err := strconv.Atoi(ipPort[1])
	if err != nil {
		log.Errorf("client tproxy post accept, src port to int err: %s", err)
		return nil, err
	}
	ipPort = strings.Split(dst.String(), ":")
	dstIp := ipPort[0]
	dstPort, err := strconv.Atoi(ipPort[1])
	if err != nil {
		log.Errorf("client tproxy post accept, dst port to int err: %s", err)
		return nil, err
	}

	ctx := &ctx{
		srcIp:   srcIp,
		srcPort: srcPort,
		dstIp:   dstIp,
		dstPort: dstPort,
		dstTo:   dstIp + ":" + strconv.Itoa(dstPort),
	}
	staticPolicyMatch := false
	for _, policy := range conf.Policies {
		transferIpPort := strings.Split(policy.Dst, ":")
		if len(transferIpPort) != 2 {
			// TODO warning
			continue
		}
		ip := transferIpPort[0]
		port := transferIpPort[1]
		if (ip == "" || ip == dstIp) && port == ipPort[1] {
			staticPolicyMatch = true
			// static match
			if policy.Proxy != nil {
				ctx.dial = policy.Proxy
			}
			if policy.DstTo != "" {
				ctx.dstTo = policy.DstTo
			}
			break
		}
	}
	if !staticPolicyMatch {
		// manager
		return ctx, nil
	}
	if ctx.dial.TLS == nil {
		ctx.dial = &gconfig.Dial{
			Network: conf.DefaultProxy.Network,
			Addrs: []string{
				dstIp + ":" + strconv.Itoa(conf.DefaultProxy.ServerPort),
			},
			TLS: conf.DefaultProxy.TLS,
		}
	}
	return ctx, nil
}

func (client *Client) handleConn(conn net.Conn, custom interface{}) error {
	var err error
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		if err = conn.Close(); err != nil {
			log.Errorf("handle conn, close non tcp conn err: %s", err)
		}
		return err
	}
	tcpFile, err := tcpConn.File()
	if err != nil {
		log.Errorf("handle conn, get tcp file from tcp conn err: %s", err)
		if err = tcpConn.Close(); err != nil {
			log.Errorf("handle conn, close tcp conn err: %s", err)
		}
		return err
	}
	if err = tcpConn.Close(); err != nil {
		log.Errorf("handle conn, close tcp conn err: %s", err)
		if err = tcpFile.Close(); err != nil {
			log.Errorf("handle conn, close tcp file err: %s", err)
		}
		return err
	}
	mark, err := utils.GetSocketMark(tcpFile.Fd())
	if err != nil {
		log.Warnf("handle conn, get socket mark err: %s", err)
	}
	ctx := custom.(*ctx)
	if mark != 0 {

	} else {
		// mark not found, maybe fwmark_accept not enabled
	}
	return nil
}

func (client *Client) tproxyPreDial(custom interface{}) error {
	return nil
}

func (client *Client) tproxyDial(dst net.Addr, custom interface{}) (net.Conn, error) {
	ctx := custom.(*ctx)
	return utils.DialRandom(ctx.dial)
}

func (client *Client) tproxyPostDial(custom interface{}) error {
	return nil
}

func (client *Client) tproxyPreWrite(writer io.Writer, custom interface{}) error {
	ctx := custom.(*ctx)
	proto := &proto.ConduitProto{
		SrcIp:   ctx.srcIp,
		SrcPort: ctx.srcPort,
		DstIp:   ctx.dstIp,
		DstPort: ctx.dstPort,
		DstTo:   ctx.dstTo,
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
