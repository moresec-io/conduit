package network

import "github.com/moresec-io/conduit/pkg/utils"

const (
	Sysctl = "sysctl"

	Write = "-w"
	Read  = "-r"
)

func EnableFWMark() ([]byte, []byte, error) {
	accept := "net.ipv4.tcp_fwmark_accept=1"
	return utils.Cmd(Sysctl, Write, accept)
}
