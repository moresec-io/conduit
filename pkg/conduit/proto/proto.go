/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package proto

type ConduitProto struct {
	SrcIP   string
	SrcPort int
	DstIP   string
	DstPort int
	DstTo   string
}
