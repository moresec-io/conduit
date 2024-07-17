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
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"syscall"

	"github.com/jumboframes/armorigo/log"
	"github.com/jumboframes/armorigo/rproxy"
	"github.com/moresec-io/conduit/pkg/conduit/config"
	"github.com/moresec-io/conduit/pkg/conduit/proto"
	gconfig "github.com/moresec-io/conduit/pkg/config"
	"github.com/moresec-io/conduit/pkg/utils"
)

const (
	// chain
	ConduitChain = "CONDUIT"

	// ipset
	ConduitIPSetPort   = "CONDUIT_PORT"
	ConduitIPSetIPPort = "CONDUIT_IPPORT"
	ConduitIPSetIP     = "CONDUIT_IP"
)

type policy struct {
	dialConfig *utils.DialConfig
	dstTo      string
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
	ipPolicies     map[string]*policy
}

func NewClient(conf *config.Config) (*Client, error) {
	client := &Client{
		conf:           conf,
		quit:           make(chan struct{}),
		ipportPolicies: make(map[string]*policy),
		portPolicies:   make(map[int]*policy),
		ipPolicies:     make(map[string]*policy),
	}
	// listen
	ipPort := strings.Split(conf.Client.Listen, ":")
	if len(ipPort) != 2 {
		return nil, errors.New("illegal client listen addr")
	}
	port, err := strconv.Atoi(ipPort[1])
	if err != nil {
		return nil, err
	}
	client.port = port

	// default proxy
	config := &gconfig.Dial{
		Network: conf.Client.DefaultProxy.Network,
		TLS:     conf.Client.DefaultProxy.TLS,
	}
	defaultdialconfig, err := utils.ConvertConfig(config)
	if err != nil {
		return nil, err
	}

	// static policy match
	for _, configpolicy := range conf.Client.Policies {
		dst, dstTo := configpolicy.Dst, configpolicy.DstTo
		ipport := strings.Split(dst, ":")
		if len(ipport) != 2 {
			return nil, errors.New("illegal policy")
		}
		ipstr := ipport[0]
		portstr := ipport[1]
		port, err := strconv.Atoi(portstr)
		if err != nil {
			return nil, err
		}
		var po *policy
		if configpolicy.Proxy == nil {
			po = &policy{
				dialConfig: defaultdialconfig,
			}
		} else {
			dialconfig, err := utils.ConvertConfig(configpolicy.Proxy)
			if err != nil {
				return nil, err
			}
			po = &policy{
				dialConfig: dialconfig,
				dstTo:      dstTo,
			}
		}
		if ipstr == "" {
			client.portPolicies[port] = po
		} else {
			client.ipportPolicies[dst] = po
		}
	}
	return client, nil
}

func (client *Client) Work() error {
	// set up proxy
	err := client.proxy()
	if err != nil {
		return err
	}
	// clear legacies
	client.finiTables(log.LevelDebug, "flush tables before init")
	client.finiIPSet(log.LevelDebug, "destroy ipset before init")

	// set
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
		rproxy.OptionRProxyAcceptConn(client.tproxyAcceptConn),
		rproxy.OptionRProxyPostAccept(client.tproxyPostAccept),
		rproxy.OptionRProxyPreDial(client.tproxyPreDial),
		rproxy.OptionRProxyPostDial(client.tproxyPostDial),
		rproxy.OptionRProxyPreWrite(client.tproxyPreWrite),
		rproxy.OptionRProxyReplaceDst(client.tproxyReplaceDst),
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
	client.finiTables(log.LevelWarn, "client fini tables")
	client.finiIPSet(log.LevelWarn, "client fini ipset")
}

type ctx struct {
	// connection info
	srcIp   string // source ip
	srcPort int    // source port
	dstIp   string // real dst ip
	dstPort int    // real dst port
	dst     string // real dst in string
	mark    uint32
	// proxy info
	dial  *policy // proxy
	dstTo string  // dst after proxy
}

const (
	SO_ORIGINAL_DST = 80
)

func (client *Client) tproxyAcceptConn(conn net.Conn) ([]interface{}, error) {
	var err error
	var meta []interface{}
	// findout original dst
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		if err = conn.Close(); err != nil {
			log.Errorf("close non tcp conn err: %s", err)
		}
		return nil, err
	}

	tcpFile, err := tcpConn.File()
	if err != nil {
		log.Errorf("get tcp file from tcp conn err: %s", err)
		if err = tcpConn.Close(); err != nil {
			log.Errorf("close tcp conn err: %s", err)
		}
		return nil, err
	}
	if err = tcpConn.Close(); err != nil {
		log.Errorf("close tcp conn err: %s", err)
		if err = tcpFile.Close(); err != nil {
			log.Errorf("close tcp file err: %s", err)
		}
		return nil, err
	}
	// mark
	mark, err := utils.GetSocketMark(tcpFile.Fd())
	if err != nil {
		log.Warnf("handle conn, get socket mark err: %s", err)
	}
	// original dst
	mreq, err := syscall.GetsockoptIPv6Mreq(
		int(tcpFile.Fd()),
		syscall.IPPROTO_IP,
		SO_ORIGINAL_DST)
	if err != nil {
		log.Errorf("get sock opt ipv6 mreq err: %s", err)
		if err = tcpFile.Close(); err != nil {
			log.Errorf("close tcp file err: %s", err)
		}
		return nil, err
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
		return nil, err
	}
	if err = tcpFile.Close(); err != nil {
		log.Errorf("close tcp file err: %s", err)
	}

	leftConn := fileConn.(*net.TCPConn)
	meta = append(meta, leftConn, mark, originalDst)
	return meta, nil
}

func (client *Client) tproxyReplaceDst(_ net.Conn, meta ...interface{}) (net.Addr, net.Conn, error) {
	if len(meta) != 3 {
		return nil, nil, errors.New("illegal meta")
	}
	return meta[2].(net.Addr), meta[0].(net.Conn), nil
}

func (client *Client) tproxyPostAccept(src, dst net.Addr, meta ...interface{}) (interface{}, error) {
	log.Debugf("client tproxy post accept, src: %s, dst: %s", src.String(), dst.String())
	if len(meta) != 3 {
		return nil, errors.New("illegal meta")
	}

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
		dst:     dst.String(),
		dstTo:   dstIp + ":" + strconv.Itoa(dstPort),
	}

	mark := meta[1].(uint32)
	var policy *policy
	var ok bool
	switch mark {
	case uint32(config.MarkIpsetIP):
		// manager policy
		policy, ok = client.ipPolicies[ctx.dstIp]
		if !ok {
			return nil, errors.New("policy not found")
		}
		ctx.dial = policy
	case uint32(config.MarkIpsetIPPort):
		policy, ok = client.ipportPolicies[ctx.dst]
		if !ok {
			return nil, errors.New("policy not found")
		}
		ctx.dial = policy
	case uint32(config.MarkIpsetPort):
		policy, ok = client.portPolicies[ctx.dstPort]
		if !ok {
			return nil, errors.New("policy not found")
		}
		ctx.dial = policy
	default:
		// failed to get mask, maybe fwmark_accept not enabled, we must iterate policies
		policy, ok = client.ipPolicies[ctx.dstIp]
		if ok {
			ctx.dial = policy
			break
		}
		policy, ok = client.ipportPolicies[ctx.dst]
		if ok {
			ctx.dial = policy
			break
		}
		policy, ok = client.portPolicies[ctx.dstPort]
		if ok {
			ctx.dial = policy
			break
		}
		return nil, errors.New("policy not found")
	}
	if policy.dstTo != "" {
		ctx.dstTo = policy.dstTo
	}
	return ctx, nil
}

func (client *Client) tproxyPreDial(custom interface{}) error {
	return nil
}

func (client *Client) tproxyDial(dst net.Addr, custom interface{}) (net.Conn, error) {
	ctx := custom.(*ctx)
	return utils.DialRandomWithConfig(ctx.dial.dialConfig)
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
