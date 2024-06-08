/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Austin Zhai
 * All rights reserved.
 */
package sigaction

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/moresec-io/conduit/pkg/log"
)

var (
	ReservedFiniSignals = []os.Signal{
		os.Kill,
		os.Interrupt,
		syscall.SIGTERM,
	}
)

type Notifier interface {
	Notify(os.Signal)
}

type SignalOption func(*Signal)

func OptionSignalCancel(cancel context.CancelFunc) SignalOption {
	return func(sig *Signal) {
		sig.cancels = append(sig.cancels, cancel)
	}
}

type Signal struct {
	mu            sync.RWMutex
	sgCh          chan os.Signal
	cancels       []context.CancelFunc
	notifications map[os.Signal][]Notifier
}

func NewSignal(options ...SignalOption) *Signal {
	sig := &Signal{
		sgCh:          make(chan os.Signal, 1),
		cancels:       []context.CancelFunc{},
		notifications: make(map[os.Signal][]Notifier),
	}
	for _, option := range options {
		option(sig)
	}
	signal.Notify(sig.sgCh, ReservedFiniSignals...)
	return sig
}

func (sig *Signal) Add(sg os.Signal, nts ...Notifier) {
	sig.mu.Lock()
	defer sig.mu.Unlock()

	for _, reserved := range ReservedFiniSignals {
		if sg == reserved {
			return
		}
	}

	storedNts, ok := sig.notifications[sg]
	if !ok {
		storedNts = []Notifier{}
		storedNts = append(storedNts, nts...)
		sig.notifications[sg] = storedNts
	} else {
		storedNts = append(storedNts, nts...)
	}
	signal.Notify(sig.sgCh, sg)
}

func (sig *Signal) Wait(ctx context.Context) {
	for {
		select {
		case sg := <-sig.sgCh:
			log.Infof("signal::wait | got signal: %s", sg.String())
			for _, reserved := range ReservedFiniSignals {
				// only call once, the second signal will be handled by os
				if sg == reserved {
					signal.Reset()
					for _, cancel := range sig.cancels {
						cancel()
					}
					goto QUIT
				}
			}
			sig.mu.RLock()
			nts, ok := sig.notifications[sg]
			sig.mu.RUnlock()
			if ok {
				for _, nt := range nts {
					nt.Notify(sg)
				}
			}

		case <-ctx.Done():
			log.Infof("signal::wait | ctx done")
			goto QUIT
		}
	}
QUIT:
	log.Infof("signal::wait | quit")
}
