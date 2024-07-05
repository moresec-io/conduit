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
	"github.com/moresec-io/conduit/pkg/agent/config"
	"github.com/moresec-io/conduit/pkg/agent/proto"
	gconfig "github.com/moresec-io/conduit/pkg/config"
	"github.com/moresec-io/conduit/pkg/tproxy"
	"github.com/moresec-io/conduit/pkg/utils"
)

const (
	ConduitChain       = "CONDUIT"
	ConduitIPSetPort   = "CONDUIT_PORT"
	ConduitIPSetIPPort = "CONDUIT_IPPORT"
)

type Client struct {
	conf *config.Config
	rp   *rproxy.RProxy
	quit chan struct{}
	// listen port
	port int
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
	srcIp   string        //源ip
	srcPort int           //源port
	dstIp   string        //原始目的ip
	dstPort int           //原始目的port
	dial    *gconfig.Dial // proxy
	dst     string        // 代理后地址
}

func (client *Client) tproxyPostAccept(src, dst net.Addr) (interface{}, error) {
	log.Debugf("client tproxy post accept, src: %s, dst: %s", src.String(), dst.String())

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
		dial: &gconfig.Dial{
			Network: client.conf.Client.DefaultProxy.Network,
			Addrs: []string{
				dstIp + ":" + strconv.Itoa(client.conf.Client.DefaultProxy.ServerPort),
			},
			TLS: client.conf.Client.DefaultProxy.TLS,
		},
		dst: dstIp + ":" + strconv.Itoa(dstPort),
	}
	for _, policy := range client.conf.Client.Policies {
		transferIpPort := strings.Split(policy.Dst, ":")
		if transferIpPort[0] == "" || transferIpPort[0] == dstIp {
			if transferIpPort[1] == ipPort[1] {
				// match
				if policy.Proxy != nil {
					ctx.dial = policy.Proxy
				}
				if policy.DstTo != "" {
					ctx.dst = policy.DstTo
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
	return utils.DialRandom(ctx.dial)
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
