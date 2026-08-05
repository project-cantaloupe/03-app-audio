package health

import (
	"context"
	"sync/atomic"
)

type Checker interface {
	Ping(context.Context) error
}

type Probe struct {
	ready atomic.Bool
}

func (p *Probe) SetReady(value bool) { p.ready.Store(value) }
func (p *Probe) Ready() bool         { return p.ready.Load() }
