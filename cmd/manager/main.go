package main

import (
	"context"

	"github.com/jumboframes/armorigo/sigaction"
	"github.com/moresec-io/conduit/pkg/manager"
	"github.com/moresec-io/conduit/pkg/manager/service"
)

func main() {
	container, err := manager.BuildContainer()
	if err != nil {
		return
	}
	err = container.Invoke(func(cm *service.ConduitManager) {
		cm.Serve()
	})
	if err != nil {
		return
	}

	sig := sigaction.NewSignal()
	sig.Wait(context.TODO())

	container.Invoke(func(cm *service.ConduitManager) {
		cm.Close()
	})
}
