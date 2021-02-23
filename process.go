package process

import (
	"context"
	"sync"
)

type groupKeyType int

const groupKey groupKeyType = iota

type Process interface {
	Done() <-chan struct{}
	Err() error
}

type Group interface {
	context.Context
	Run(func(context.Context) error) Process
	Loop(func(context.Context) error) Process
	Done() <-chan struct{}
	Join() <-chan struct{}
	Err() error
}

type process struct {
	Context
	cancel CancelFunc
}

type group struct {
	Context
	cancel CancelFunc

	join      chan struct{}
	mu        sync.Mutex
	processes []Process
}

func NewGroup(ctx context.Context) Group {
	groupCtx, groupCancel := WithCancel(ctx)
	g := &group{
		Context: groupCtx,
		cancel:  groupCancel,
		join:    make(chan struct{}),
	}
	go g.joinAfterDone()
	return g
}

func (g *group) joinAfterDone() {
	<-g.Context.Done()
	g.mu.Lock()
	processes := g.processes
	g.mu.Unlock()
	for _, process := range processes {
		<-process.Done()
	}
	close(g.join)
}

func (g *group) Join() <-chan struct{} {
	return g.join
}

func (g *group) Run(f func(ctx context.Context) error) Process {
	g.mu.Lock()
	defer g.mu.Unlock()
	processCtx, processCancel := NewContext()
	proc := &process{
		Context: processCtx,
		cancel:  processCancel,
	}

	select {
	case <-g.Context.Done():
		processCancel(g.Context.Err())
	default:
		go func() {
			err := f(g)
			if err != nil {
				g.cancel(err)
				processCancel(err)
			}
		}()
		g.processes = append(g.processes, proc)
	}

	return proc
}

func (g *group) Loop(f func(ctx context.Context) error) Process {
	g.mu.Lock()
	defer g.mu.Unlock()
	processCtx, processCancel := NewContext()
	proc := &process{
		Context: processCtx,
		cancel:  processCancel,
	}

	select {
	case <-g.Context.Done():
		processCancel(g.Context.Err())
	default:
		go func() {
			for {
				err := f(g)
				if err != nil {
					g.cancel(err)
					processCancel(err)
					return
				}
			}
		}()
		g.processes = append(g.processes, proc)
	}

	return proc
}

func (g *group) Value(key interface{}) interface{} {
	if key == groupKey {
		return g
	}
	return g.Value(key)
}

type traceGroup struct {
	Group
	trace Context
}

func (t traceGroup) Err() error {
	return t.trace.Err()
}

func GroupFromContext(ctx context.Context) Group {
	if g, ok := ctx.(*group); ok {
		return traceGroup{Group: g, trace: WithTrace(g)}
	}
	if g, ok := ctx.Value(groupKey).(*group); ok {
		return traceGroup{Group: g, trace: WithTrace(g)}
	}
	return NewGroup(ctx)
}
