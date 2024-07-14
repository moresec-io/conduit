package utils

const (
	Sysctl = "sysctl"

	Write = "-w"
	Read  = "-r"
)

func EnableFWMark() ([]byte, []byte, error) {
	accept := "net.ipv4.tcp_fwmark_accept=1"
	return Cmd(Sysctl, Write, accept)
}
