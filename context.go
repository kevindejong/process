package process

import (
	"context"
	"sync"
	"time"
)

type Context context.Context
type CancelFunc func(error)

type customContext struct {
	mu   sync.Mutex
	done chan struct{}
	err  error

	parent context.Context
	frame  *Frame //the call frame when the context is created
}

// NewContext returns a new context with a custom error type
func NewContext() (Context, CancelFunc) {
	return WithCancel(context.Background())
}

func WithCancel(ctx context.Context) (Context, CancelFunc) {
	c := &customContext{
		done:   make(chan struct{}),
		parent: ctx,
		frame:  externalFrame(),
	}
	go c.propagateCancel()
	return c, c.cancel
}

func (c *customContext) propagateCancel() {
	select {
	case <-c.parent.Done():
		c.cancel(nil)
	case <-c.done:
		return
	}
}

func (c *customContext) cancel(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.done:
		return
	default:
	}
	// always give preference to the parent context
	if c.parent != nil {
		select {
		case <-c.parent.Done():
			c.err = withContextFrame(c.parent.Err(), c.frame)
			close(c.done)
			return
		default:
		}
	}
	c.err = withContextFrame(wrapContextFrame(&errContextCanceled{cause: err}), c.frame)
	close(c.done)
}

func (c *customContext) Done() <-chan struct{} {
	return c.done
}

func (c *customContext) Err() error {
	// if the parent is canceled, we are racing against propagateCancel so
	// wait for it to complete
	if c.parent != nil {
		select {
		case <-c.parent.Done():
			<-c.done
		default:
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.done:
		return wrapContextFrame(c.err)
	default:
		return nil
	}
}

func (c *customContext) Value(key interface{}) interface{} {
	if c.parent != nil {
		return c.parent.Value(key)
	}
	return nil
}

func (c *customContext) Deadline() (deadline time.Time, ok bool) {
	if c.parent != nil {
		return c.parent.Deadline()
	}
	return time.Time{}, false
}
