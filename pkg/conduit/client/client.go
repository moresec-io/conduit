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
	ierrors "github.com/moresec-io/conduit/pkg/conduit/errors"
	"github.com/moresec-io/conduit/pkg/conduit/proto"
	"github.com/moresec-io/conduit/pkg/conduit/repo"
	"github.com/moresec-io/conduit/pkg/conduit/syncer"
	"github.com/moresec-io/conduit/pkg/conduit/sys"
	gconfig "github.com/moresec-io/conduit/pkg/config"
	"github.com/moresec-io/conduit/pkg/network"
	gproto "github.com/moresec-io/conduit/pkg/proto"
)

const (
	// chain
	ConduitChain = "CONDUIT"

	// ipset
	ConduitIPSetPort   = "CONDUIT_PORT"
	ConduitIPSetIPPort = "CONDUIT_IPPORT"
	ConduitIPSetIP     = "CONDUIT_IP"
)

type peer struct {
	index      int
	dialConfig *network.DialConfig
}

type policy struct {
	peerDialConfig *network.DialConfig
	dstAs          string
}

type Client struct {
	conf *config.Config
	rp   *rproxy.RProxy
	quit chan struct{}
	// listen port
	port int

	// static peers
	peers map[int]*peer

	repo   repo.Repo
	syncer syncer.Syncer
}

func NewClient(conf *config.Config, syncer syncer.Syncer, rp repo.Repo) (*Client, error) {
	client := &Client{
		conf:   conf,
		quit:   make(chan struct{}),
		peers:  make(map[int]*peer),
		repo:   rp,
		syncer: syncer,
	}
	// client listen
	ipPort := strings.Split(conf.Client.Listen, ":")
	if len(ipPort) != 2 {
		return nil, ierrors.ErrIllegalClientListenAddress
	}
	port, err := strconv.Atoi(ipPort[1])
	if err != nil {
		return nil, err
	}
	client.port = port
	// clear legacies
	client.finiTables(log.LevelDebug, "flush tables before init")
	client.repo.FiniIPSet(log.LevelDebug, "destroy ipset before init")

	// set
	err = client.setIPSet()
	if err != nil {
		return nil, err
	}
	err = client.setTables()
	if err != nil {
		return nil, err
	}
	err = client.setProc()
	if err != nil {
		return nil, err
	}
	err = client.setStaticPolicies()
	if err != nil {
		return nil, err
	}
	// manager
	if conf.Manager.Enable {
		_, err := syncer.ReportClient(&gproto.ReportClientRequest{
			MachineID: conf.MachineID,
		})
		if err != nil {
			return nil, err
		}
		err = syncer.PullCluster()
		if err != nil {
			return nil, err
		}
	}

	// peers
	for _, elem := range conf.Client.Peers {
		_, ok := client.peers[elem.Index]
		if ok {
			return nil, ierrors.ErrDuplicatedPeerIndexConfigured
		}
		config := &gconfig.Dial{
			Network:   elem.Network,
			Addresses: elem.Addresses,
			TLS:       &elem.TLS,
		}
		dialConfig, err := network.ConvertDialConfig(config)
		if err != nil {
			return nil, err
		}
		client.peers[elem.Index] = &peer{
			index:      elem.Index,
			dialConfig: dialConfig,
		}
	}

	// static forward match
	for _, elem := range conf.Client.ForwardTable {
		dst, dstAs := elem.Dst, elem.DstAs
		dstIPPort := strings.Split(dst, ":")
		if len(dstIPPort) != 2 {
			return nil, errors.New("illegal policy")
		}
		dstAsIPPort := strings.Split(dstAs, ":")
		if len(dstAsIPPort) != 2 {
			return nil, errors.New("illegal policy")
		}
		ipstr := dstIPPort[0]
		portstr := dstIPPort[1]
		port, err := strconv.Atoi(portstr)
		if err != nil {
			return nil, err
		}
		peer, ok := client.peers[elem.PeerIndex]
		if !ok {
			return nil, ierrors.ErrPeerIndexNotfound
		}
		if ipstr == "" {
			client.repo.AddPortPolicy(port, &repo.Policy{
				PeerDialConfig: peer.dialConfig,
				DstAs:          dstAs,
			})
		} else {
			client.repo.AddIPPortPolicy(dst, &repo.Policy{
				PeerDialConfig: peer.dialConfig,
				DstAs:          dstAs,
			})
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
	return nil
}

func (client *Client) setStaticPolicies() error {
	for _, elem := range client.conf.Client.ForwardTable {
		transferIpPort := strings.Split(elem.Dst, ":")
		ip := transferIpPort[0]
		port, err := strconv.Atoi(transferIpPort[1])
		if err != nil {
			return err
		}
		if ip == "" {
			err = client.repo.AddIPSetPort(uint16(port))
			if err != nil {
				return err
			}
		} else {
			err = client.repo.AddIPSetIPPort(net.ParseIP(ip), uint16(port))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (client *Client) proxy() error {
	listener, err := net.Listen(client.conf.Client.Network, client.conf.Client.Listen)
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
	client.repo.FiniIPSet(log.LevelWarn, "client fini ipset")
}

type ctx struct {
	// connection info
	srcIP   string // source ip
	srcPort int    // source port
	dstIP   string // real dst ip
	dstPort int    // real dst port
	dst     string // real dst in string
	// mark    uint32
	dial  *repo.Policy // proxy
	dstAs string       // dst after proxy
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
	mark, err := network.GetSocketMark(tcpFile.Fd())
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
		srcIP:   srcIp,
		srcPort: srcPort,
		dstIP:   dstIp,
		dstPort: dstPort,
		dst:     dst.String(),
		dstAs:   dstIp + ":" + strconv.Itoa(dstPort),
	}

	mark := meta[1].(uint32)
	var policy *repo.Policy
	switch mark {
	case uint32(config.MarkIpsetIP):
		// manager policy
		policy = client.repo.GetPolicyByIP(ctx.dstIP)
		if policy == nil {
			log.Errorf("client tproxy post accept, ip: %s policy not found", ctx.dstIP)
			return nil, errors.New("policy not found")
		}
		ctx.dial = policy
	case uint32(config.MarkIpsetIPPort):
		policy = client.repo.GetPolicyByIPPort(ctx.dst)
		if policy == nil {
			log.Errorf("client tproxy post accept, ipport: %s policy not found", ctx.dst)
			return nil, errors.New("policy not found")
		}
		ctx.dial = policy
	case uint32(config.MarkIpsetPort):
		policy = client.repo.GetPolicyByPort(ctx.dstPort)
		if policy == nil {
			log.Errorf("client tproxy post accept, dstport: %v policy not found", ctx.dstPort)
			return nil, errors.New("policy not found")
		}
		ctx.dial = policy
	default:
		// failed to get mask, maybe fwmark_accept not enabled, we must iterate policies
		policy = client.repo.GetPolicyByIPPort(ctx.dst)
		if policy != nil {
			ctx.dial = policy
			break
		}
		policy = client.repo.GetPolicyByPort(ctx.dstPort)
		if policy != nil {
			ctx.dial = policy
			break
		}
		policy = client.repo.GetPolicyByIP(ctx.dstIP)
		if policy != nil {
			ctx.dial = policy
			break
		}
		log.Errorf("client tproxy post accept, ip: %s, ipport: %s, dstport: %v policy not found", ctx.dstIP, ctx.dst, ctx.dstPort)
		return nil, errors.New("policy not found")
	}
	if policy.DstAs != "" {
		ctx.dstAs = policy.DstAs
	}
	return ctx, nil
}

func (client *Client) tproxyPreDial(custom interface{}) error {
	return nil
}

func (client *Client) tproxyDial(dst net.Addr, custom interface{}) (net.Conn, error) {
	ctx := custom.(*ctx)
	config := ctx.dial.PeerDialConfig
	config.Control = sys.Control
	return network.DialRandomWithConfig(ctx.dial.PeerDialConfig)
}

func (client *Client) tproxyPostDial(custom interface{}) error {
	return nil
}

func (client *Client) tproxyPreWrite(writer io.Writer, custom interface{}) error {
	ctx := custom.(*ctx)
	proto := &proto.ConduitProto{
		SrcIP:   ctx.srcIP,
		SrcPort: ctx.srcPort,
		DstIP:   ctx.dstIP,
		DstPort: ctx.dstPort,
		DstAs:   ctx.dstAs,
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
