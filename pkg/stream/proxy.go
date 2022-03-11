package stream

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"time"

	"github.com/jumboframes/conduit/pkg/libio"
	"github.com/jumboframes/conduit/pkg/log"
)

func connectToRemote(raddr *net.TCPAddr, isTLS bool) (net.Conn, error) {
	timeout := time.Second * 10
	if isTLS {
		dialer := net.Dialer{Timeout: timeout}
		tc := tls.Config{InsecureSkipVerify: true}
		return tls.DialWithDialer(&dialer, "tcp", raddr.String(), &tc)
	}

	return net.DialTimeout("tcp", raddr.String(), timeout)
}

func streamCopy(lconn io.ReadWriteCloser, rconn io.ReadWriteCloser) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		_, _ = libio.Copy(lconn, rconn)
		_ = lconn.Close()
		_ = rconn.Close()
		wg.Done()
	}()

	go func() {
		_, _ = libio.Copy(rconn, lconn)
		_ = lconn.Close()
		_ = rconn.Close()
		wg.Done()
	}()

	wg.Wait()
}

type basicProxy struct {
	lconn, rconn net.Conn
}

func newBasicProxy(lconn, rconn net.Conn) *basicProxy {
	return &basicProxy{lconn: lconn, rconn: rconn}
}

func (p *basicProxy) Proxy() {
	defer p.lconn.Close()
	defer p.rconn.Close()
	//p.rconn.SetDeadline(time.Now().Add(time.Minute * 5))
	log.Debugf("basicProxy::Proxy | %s to %s starts",
		p.lconn.LocalAddr(), p.rconn.RemoteAddr())
	defer log.Debugf("basicProxy::Proxy | %s to %s ends",
		p.lconn.LocalAddr(), p.rconn.RemoteAddr())
	streamCopy(p.lconn, p.rconn)
}

type TCPProxy struct {
	*basicProxy
}

func NewTCPProxy(lconn net.Conn, raddr *net.TCPAddr, isTLS bool) (*TCPProxy, error) {
	rconn, err := connectToRemote(raddr, isTLS)
	if err != nil {
		return nil, err
	}

	return &TCPProxy{&basicProxy{lconn: lconn, rconn: rconn}}, nil
}
