package proxy

const (
	ProxyModeRaw  = "raw"
	ProxyModeTls  = "tls"
	ProxyModeMTls = "mtls"
)

type MSProxyProto struct {
	SrcIp         string
	SrcPort       int
	DstIpOrigin   string
	DstPortOrigin int
}
