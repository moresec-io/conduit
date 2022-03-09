package nf_wrapper

import (
	"bytes"
	"os/exec"
	"sync/atomic"
)

var (
	stdOutBuf int32 = 512
	stdErrBuf int32 = 1024
)

func SetStdOutBuf(buf int32) {
	atomic.StoreInt32(&stdOutBuf, buf)
}

func SetStdErrBuf(buf int32) {
	atomic.StoreInt32(&stdErrBuf, buf)
}

func GetStdOutBuf() int32 {
	return atomic.LoadInt32(&stdOutBuf)
}

func GetStdErrBuf() int32 {
	return atomic.LoadInt32(&stdErrBuf)
}

func Cmd(name string, arg ...string) ([]byte, []byte, error) {
	cmd := exec.Command(name, arg...)
	infoO := new(bytes.Buffer)
	infoE := new(bytes.Buffer)
	cmd.Stdout = infoO
	cmd.Stderr = infoE
	err := cmd.Run()
	if err != nil {
		return infoO.Bytes(), infoE.Bytes(), err
	}
	return infoO.Bytes(), infoE.Bytes(), nil
}
