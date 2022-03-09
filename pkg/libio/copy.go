package libio

import "io"

func Copy(dst io.Writer, src io.Reader) (int64, error) {
	buf := DefaultAllocator.Get(32 * 1024)
	defer DefaultAllocator.Put(buf)
	return io.CopyBuffer(dst, src, buf)
}
