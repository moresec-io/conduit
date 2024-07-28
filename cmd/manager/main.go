package manager

import (
	"context"

	"github.com/jumboframes/armorigo/log"
	"github.com/jumboframes/armorigo/sigaction"
	"github.com/moresec-io/conduit/pkg/manager"
)

func main() {
	container, err := manager.BuildContainer()
	if err != nil {
		log.Errorf("manager build container err: %s", err)
		return
	}
	container.Invoke(func() {})

	sig := sigaction.NewSignal()
	sig.Wait(context.TODO())

	container.Invoke(func() {})
}
