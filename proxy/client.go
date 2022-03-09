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
	"github.com/jumboframes/conduit/pkg/tproxy"
)

type Client struct {
	conf  *conduit.Config
	tp    *tproxy.TProxy
	certs []tls.Certificate
}

func NewClient(conf *conduit.Config) (*Client, error) {
	client := &Client{conf: conf}
	if conf.Client.Proxy.Mode == ProxyModeMTls {
		cert, err := tls.LoadX509KeyPair(conf.Client.Cert.CertFile,
			conf.Client.Cert.KeyFile)
		if err != nil {
			return nil, err
		}
		client.certs = []tls.Certificate{cert}
	}
	return client, nil
}

func (client *Client) Proxy() error {
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
}

type ctx struct {
	src, dst   net.Addr
	srcIp      string
	srcPort    int
	dstIp      string
	dstPort    int
	serverPort int
}

func (client *Client) tproxyPostAccept(src, dst net.Addr) (interface{}, error) {
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
		src:        src,
		dst:        dst,
		srcIp:      srcIp,
		srcPort:    srcPort,
		dstIp:      dstIp,
		dstPort:    dstPort,
		serverPort: client.conf.Client.Proxy.ServerPort,
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
	return net.DialTimeout("tcp4",
		ctx.dstIp+":"+strconv.Itoa(ctx.serverPort),
		time.Duration(timeout)*time.Second)
}

func (client *Client) tlsDial(pipe *tproxy.Pipe, ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	dialer := net.Dialer{Timeout: time.Duration(timeout) * time.Second}

	return tls.DialWithDialer(&dialer, "tcp4",
		ctx.dstIp+":"+strconv.Itoa(ctx.serverPort),
		&tls.Config{InsecureSkipVerify: true})
}

func (client *Client) mtlsDial(pipe *tproxy.Pipe, ctx *ctx) (net.Conn, error) {
	timeout := client.conf.Client.Proxy.Timeout
	dialer := net.Dialer{Timeout: time.Duration(timeout) * time.Second}
	return tls.DialWithDialer(&dialer, "tcp4",
		ctx.dstIp+":"+strconv.Itoa(ctx.serverPort),
		&tls.Config{InsecureSkipVerify: true, Certificates: client.certs})
}

func (client *Client) tproxyPostDial(pipe *tproxy.Pipe, custom interface{}) error {
	// TODO
	return nil
}

func (client *Client) tproxyPreWrite(writer io.Writer, pipe *tproxy.Pipe, custom interface{}) error {
	ctx := custom.(*ctx)
	proto := &MSProxyProto{
		SrcIp:         ctx.srcIp,
		SrcPort:       ctx.srcPort,
		DstIpOrigin:   ctx.dstIp,
		DstPortOrigin: ctx.dstPort,
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
