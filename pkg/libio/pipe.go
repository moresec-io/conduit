package libio

import (
	"context"
	"golang.org/x/sync/errgroup"
	"io"
)

type Pipe struct {
	Pool BufferPool
}

type BufferPool interface {
	Get(size int) []byte
	Put([]byte) error
}

func (p *Pipe) Copy(dst io.Writer, src io.Reader) error {
	var buf []byte
	if p.Pool != nil {
		buf = p.Pool.Get(32 * 1024)
		defer p.Pool.Put(buf)
	}
	_, err := io.CopyBuffer(dst, src, buf)
	return err
}

func (p *Pipe) Pipe(src1, src2 io.ReadWriteCloser) error {
	wg, _ := errgroup.WithContext(context.Background())
	wg.Go(func() error {
		defer func() {
			_ = src1.Close()
			_ = src2.Close()
		}()
		return p.Copy(src1, src2)
	})
	wg.Go(func() error {
		defer func() {
			_ = src1.Close()
			_ = src2.Close()
		}()
		return p.Copy(src2, src1)
	})
	return wg.Wait()
}
