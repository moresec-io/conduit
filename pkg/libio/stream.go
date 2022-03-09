package libio

import (
	"io"
	"time"
)

var gPipe = &Pipe{
	Pool: DefaultAllocator,
}

type BufWithTimeout interface {
	io.ReadWriteCloser
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

type Stream struct {
	BufWithTimeout
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func NewStream(buf BufWithTimeout, readTimeout, writeTimeout time.Duration) *Stream {
	return &Stream{
		BufWithTimeout: buf,
		readTimeout:    readTimeout,
		writeTimeout:   writeTimeout,
	}
}

func (s *Stream) Read(p []byte) (int, error) {
	if s.readTimeout != 0 {
		_ = s.SetReadDeadline(time.Now().Add(s.readTimeout))
		defer s.SetReadDeadline(time.Time{})
	}
	return s.BufWithTimeout.Read(p)
}

func (s *Stream) Write(p []byte) (int, error) {
	if s.writeTimeout != 0 {
		_ = s.SetWriteDeadline(time.Now().Add(s.writeTimeout))
		defer s.SetWriteDeadline(time.Time{})
	}
	return s.BufWithTimeout.Write(p)
}

func (s *Stream) Pipe(s2 *Stream) error {
	return gPipe.Pipe(s, s2)
}
