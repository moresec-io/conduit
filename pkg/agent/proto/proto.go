/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package proto

type ConduitProto struct {
	SrcIp   string
	SrcPort int
	DstIp   string
	DstPort int
	Dst     string
}
