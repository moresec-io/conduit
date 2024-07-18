/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package main

import (
	"context"

	"github.com/jumboframes/armorigo/sigaction"
	conduit "github.com/moresec-io/conduit/pkg/conduit"
	//_ "net/http/pprof"
)

func main() {
	conduit, err := conduit.NewConduit()
	if err != nil {
		return
	}
	conduit.Run()

	sig := sigaction.NewSignal()
	sig.Wait(context.TODO())

	conduit.Close()
}
