package libio

import (
	"github.com/xtaci/smux"
	_ "unsafe"
)

//go:linkname DefaultAllocator github.com/xtaci/smux.defaultAllocator
var DefaultAllocator *smux.Allocator