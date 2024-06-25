/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package proxy

const (
	ProxyModeRaw  = "raw"
	ProxyModeTls  = "tls"
	ProxyModeMTls = "mtls"
)

type ConduitProto struct {
	SrcIp   string
	SrcPort int
	DstIp   string
	DstPort int
	Proxy   string
	Dst     string
}
