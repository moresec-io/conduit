package stream

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/moresec-io/conduit/pkg/log"
)

func connectToRemote(raddr *net.TCPAddr, isTLS bool,
	control func(network, address string, conn syscall.RawConn) error) (net.Conn, error) {

	timeout := time.Second * 10
	dialer := net.Dialer{
		Timeout: timeout,
		Control: control,
	}
	if isTLS {
		tc := tls.Config{InsecureSkipVerify: true}
		return tls.DialWithDialer(&dialer, "tcp", raddr.String(), &tc)
	}

	return dialer.Dial("tcp", raddr.String())
}

func streamCopy(lconn io.ReadWriteCloser, rconn io.ReadWriteCloser) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		//_, err := libio.Copy(lconn, rconn)
		_, err := io.Copy(lconn, rconn)
		if err != nil {
			log.Debugf("streamCopy | libio Copy right to left err: %s", err)
		}
		_ = lconn.Close()
		_ = rconn.Close()
		wg.Done()
	}()

	go func() {
		//_, err := libio.Copy(rconn, lconn)
		_, err := io.Copy(rconn, lconn)
		if err != nil {
			log.Debugf("streamCopy | libio Copy left to right err: %s", err)
		}
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

func NewTCPProxy(lconn net.Conn, raddr *net.TCPAddr, isTLS bool,
	control func(network, address string, conn syscall.RawConn) error) (*TCPProxy, error) {
	rconn, err := connectToRemote(raddr, isTLS, control)
	if err != nil {
		return nil, err
	}

	return &TCPProxy{&basicProxy{lconn: lconn, rconn: rconn}}, nil
}
