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
	"github.com/moresec-io/conduit/pkg/agent"
	//_ "net/http/pprof"
)

func main() {
	agent, err := agent.NewAgent()
	if err != nil {
		return
	}
	agent.Run()

	sig := sigaction.NewSignal()
	sig.Wait(context.TODO())

	agent.Close()
}
