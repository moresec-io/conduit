/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Austin Zhai
 * All rights reserved.
 */
package proxy

const (
	ProxyModeRaw  = "raw"
	ProxyModeTls  = "tls"
	ProxyModeMTls = "mtls"
)

type MSProxyProto struct {
	SrcIp   string
	SrcPort int
	DstIp   string
	DstPort int
	Proxy   string
	Dst     string
}
