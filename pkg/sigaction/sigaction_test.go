package sigaction

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

func TestSignal(t *testing.T) {
	signal.Reset()
	foo := &foo{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		fmt.Println("sub routine quit")
	}()

	ReservedFiniSignal = syscall.SIGTERM
	sig := NewSignal(OptionSignalCancel(cancel))
	sig.Add(syscall.SIGINT, foo)
	sig.Wait(ctx)
}

type foo struct{}

func (foo *foo) Notify(sg os.Signal) {
	fmt.Println("notification: ", sg.String())
}
