/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package tproxy

import (
	"context"
	"fmt"
	"io"
	"testing"
)

func Test_IPv4PortValid(t *testing.T) {
	if IPv4PortValid("192.168.120.4:-1") {
		t.Errorf("IPv4PortValid port -1 err")
		return
	}
	if IPv4PortValid("192.168.120.4:65536") {
		t.Errorf("IPv4PortValid port 65536 err")
		return
	}
}

func Test_TProxy(t *testing.T) {
	tproxy, err := NewTProxy(context.Background(),
		"0.0.0.0:2432",
		OptionTProxyPreDial(PreDialTest),
		OptionTProxyPreWrite(PreWriteTest))
	if err != nil {
		t.Error(err)
		return
	}

	tproxy.Listen()
}

//func Test_TProxyDirect(t *testing.T) {
//	tproxy, err := NewTProxy("0.0.0.0:2432")
//	if err != nil {
//		t.Error(err)
//		return
//	}
//
//	tproxy.Listen()
//}

func PreDialTest(customer interface{}) error {
	return nil
}

func PreWriteTest(writer io.Writer, custom interface{}) error {
	_, err := writer.Write([]byte(
		fmt.Sprintf("github.com/moresec-io/conduit %s cpf_forward\n", "192.168.110.205:22")))
	return err
}
