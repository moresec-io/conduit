package tproxy

import (
	"context"
	"fmt"
	"io"
	"github.com/jumboframes/conduit/pkg/log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

const (
	SO_ORIGINAL_DST = 80
)

type OptionTProxy func(tproxy *TProxy) error

//如果返回err则断开连接
type PostAccept func(src net.Addr, dst net.Addr) (interface{}, error)
type PreWrite func(writer io.Writer, pipe *Pipe, custom interface{}) error
type PreDial func(pipe *Pipe, custom interface{}) error
type PostDial func(pipe *Pipe, custom interface{}) error
type Dial func(pipe *Pipe, custom interface{}) (net.Conn, error)

func OptionTProxyPostAccept(postAccept PostAccept) OptionTProxy {
	return func(tproxy *TProxy) error {
		tproxy.postAccept = postAccept
		return nil
	}
}

func OptionTProxyPreWrite(preWrite PreWrite) OptionTProxy {
	return func(tproxy *TProxy) error {
		tproxy.preWrite = preWrite
		return nil
	}
}

func OptionTProxyPreDial(preDial PreDial) OptionTProxy {
	return func(tproxy *TProxy) error {
		tproxy.preDial = preDial
		return nil
	}
}

func OptionTProxyPostDial(postDial PostDial) OptionTProxy {
	return func(tproxy *TProxy) error {
		tproxy.postDial = postDial
		return nil
	}
}

func OptionTProxyDial(dial Dial) OptionTProxy {
	return func(tproxy *TProxy) error {
		tproxy.dial = dial
		return nil
	}
}

type TProxy struct {
	listener net.Listener

	//context for acceptor, conns
	ctx    context.Context
	cancel context.CancelFunc

	//hooks
	postAccept PostAccept
	preWrite   PreWrite
	preDial    PreDial
	postDial   PostDial
	dial       Dial

	//holder
	pipes map[string]*Pipe
	mutex sync.RWMutex

	//enable
	enable uint32
}

func NewTProxy(ctx context.Context, localAddr string, options ...OptionTProxy) (*TProxy, error) {
	listener, err := net.Listen("tcp4", localAddr)
	if err != nil {
		return nil, err
	}

	tproxy := &TProxy{}
	for _, option := range options {
		if err = option(tproxy); err != nil {
			return nil, err
		}
	}

	tproxy.listener = listener
	tproxy.ctx, tproxy.cancel = context.WithCancel(ctx)

	return tproxy, nil
}

func (tproxy *TProxy) Listen() {
	for {
		//利用TProxy::Close来保障退出
		conn, err := tproxy.listener.Accept()
		if err != nil {
			log.Errorf("tproxy accept conn err: %s, quiting", err)
			return
		}

		if atomic.LoadUint32(&tproxy.enable) == 0 {
			if err = conn.Close(); err != nil {
				log.Errorf("close conn err: %s, while tproxy enable", err)
			}
			continue
		}

		originalDst, leftConn, err := acquireOriginalDst(conn)
		if err != nil {
			log.Errorf("acquire original dst addr err: %s", err)
			continue
		}

		var custom interface{}
		//post accept
		if tproxy.postAccept != nil {
			if custom, err = tproxy.postAccept(leftConn.RemoteAddr(), originalDst); err != nil {
				log.Errorf("post accept return err: %s", err)
				if err = leftConn.Close(); err != nil {
					log.Errorf("close left conn err: %s", err)
				}
				continue
			}
		}

		pipe := &Pipe{
			tproxy:      tproxy,
			Src:         leftConn.RemoteAddr(),
			OriginalDst: originalDst,
			custom:      custom,
			leftConn:    leftConn}

		go pipe.proxy()
	}
}

func (tproxy *TProxy) Enable(enable uint32) {
	atomic.StoreUint32(&tproxy.enable, enable)
}

func (tproxy *TProxy) addPipe(pipe *Pipe) {
	tproxy.mutex.Lock()
	key := pipe.Src.String() + pipe.OriginalDst.String()
	tproxy.pipes[key] = pipe
	tproxy.mutex.Unlock()
}

func (tproxy *TProxy) delPipe(pipe *Pipe) {
	tproxy.mutex.Lock()
	key := pipe.Src.String() + pipe.OriginalDst.String()
	delete(tproxy.pipes, key)
	tproxy.mutex.Unlock()
}

func (tproxy *TProxy) Close() {
	err := tproxy.listener.Close()
	if err != nil {
		log.Errorf("tproxy listener close err: %s, routine continued", err)
	}
	tproxy.cancel()
}

type Pipe struct {
	//father
	tproxy *TProxy

	Src         net.Addr
	OriginalDst net.Addr //原始目的地址

	//custom
	custom interface{}

	leftConn  net.Conn
	rightConn net.Conn
}

func (pipe *Pipe) proxy() {
	defer pipe.tproxy.delPipe(pipe)

	//预先连接
	if pipe.tproxy.preDial != nil {
		if err := pipe.tproxy.preDial(pipe, pipe.custom); err != nil {
			_ = pipe.leftConn.Close()
			return
		}
	}

	log.Debugf("tpProxy debug: start dial")
	var err error
	dial := rawSyscallDial
	if pipe.tproxy.dial != nil {
		dial = pipe.tproxy.dial
	}
	pipe.rightConn, err = dial(pipe, pipe.custom)
	if err != nil {
		log.Errorf("tpProxy dial error: %v", err)
		_ = pipe.leftConn.Close()
		return
	}

	//连接后
	if pipe.tproxy.postDial != nil {
		if err := pipe.tproxy.postDial(pipe, pipe.custom); err != nil {
			log.Errorf("tpProxy dial error: %v", err)
			_ = pipe.leftConn.Close()
			_ = pipe.rightConn.Close()
			return
		}
	}

	//预先写
	if pipe.tproxy.preWrite != nil {
		if err = pipe.tproxy.preWrite(pipe.rightConn, pipe, pipe.custom); err != nil {
			_ = pipe.leftConn.Close()
			_ = pipe.rightConn.Close()
			return
		}
	}

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		defer wg.Done()

		_, _ = io.Copy(pipe.leftConn, pipe.rightConn)
		_ = pipe.rightConn.Close()
		_ = pipe.leftConn.Close()
	}()

	go func() {
		defer wg.Done()

		_, _ = io.Copy(pipe.rightConn, pipe.leftConn)
		_ = pipe.rightConn.Close()
		_ = pipe.leftConn.Close()
	}()

	exist := make(chan struct{})
	defer close(exist)

	go func() { // 优雅退出
		select {
		case <-exist:
		case <-pipe.tproxy.ctx.Done():
			log.Debugf("force pipe done from parent")
			_ = pipe.leftConn.Close()
			_ = pipe.rightConn.Close()
		}
	}()
	wg.Wait()

	log.Debugf("close right and left conn")
	_ = pipe.rightConn.Close()
	_ = pipe.leftConn.Close()
}

func (pipe *Pipe) dial() (net.Conn, error) {
	var rightConn net.Conn
	var err error

	return rightConn, err
}

//varify ipv4:port
func IPv4PortValid(ipPort string) bool {
	parts := strings.Split(ipPort, ":")
	if len(parts) != 2 {
		return false
	}
	ip := net.ParseIP(parts[0])
	if ip.To4() == nil {
		return false
	}

	_, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return false
	}
	return true
}

//注意没有拿到originalDst就会关闭连接
func acquireOriginalDst(conn net.Conn) (net.Addr, *net.TCPConn, error) {
	var err error
	//需要从conn中找到原始dst addr

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		if err = conn.Close(); err != nil {
			log.Errorf("close non tcp conn err: %s", err)
		}
		return nil, nil, err
	}
	tcpFile, err := tcpConn.File()
	if err != nil {
		log.Errorf("get tcp file from tcp conn err: %s", err)
		if err = tcpConn.Close(); err != nil {
			log.Errorf("close tcp conn err: %s", err)
		}
		return nil, nil, err
	}
	if err = tcpConn.Close(); err != nil {
		log.Errorf("close tcp conn err: %s", err)
		if err = tcpFile.Close(); err != nil {
			log.Errorf("close tcp file err: %s", err)
		}
		return nil, nil, err
	}

	//the real shit
	mreq, err := syscall.GetsockoptIPv6Mreq(
		int(tcpFile.Fd()),
		syscall.IPPROTO_IP,
		SO_ORIGINAL_DST)
	if err != nil {
		log.Errorf("get sock opt ipv6 mreq err: %s", err)
		if err = tcpFile.Close(); err != nil {
			log.Errorf("close tcp file err: %s", err)
		}
		return nil, nil, err
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
		return nil, nil, err
	}
	if err = tcpFile.Close(); err != nil {
		log.Errorf("close tcp file err: %s", err)
	}

	leftConn := fileConn.(*net.TCPConn)
	return originalDst, leftConn, nil
}
