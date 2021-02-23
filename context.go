package process

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type errContextCanceled struct {
	cause error
}

func (e *errContextCanceled) Error() string {
	cause := "<nil>"
	if e.cause != nil {
		cause = e.cause.Error()
	}
	return fmt.Sprintf("context canceled: %s", cause)
}

func (e *errContextCanceled) Cause() error {
	return e.cause
}

func (e *errContextCanceled) Unwrap() error {
	return e.cause
}

type customContext struct {
	parent context.Context

	mu   sync.Mutex
	done chan struct{}
	err  error
}

type Context context.Context
type CancelFunc func(error)

// NewContext returns a new context with a custom error type
func NewContext() (Context, CancelFunc) {
	return WithCancel(context.Background())
}

func WithCancel(ctx context.Context) (Context, CancelFunc) {
	c := &customContext{
		parent: ctx,
		done:   make(chan struct{}),
	}
	go c.propagateCancel()
	return WithTrace(c), c.cancel
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
	// prevent multiple cancels from panicking
	select {
	case <-c.done:
		return
	default:
	}
	// always give preference to the parent context
	select {
	case <-c.parent.Done():
		c.err = c.parent.Err()
	default:
		c.err = WrapTrace(&errContextCanceled{cause: err})
	}
	close(c.done)
}

func (c *customContext) Done() <-chan struct{} {
	return c.done
}

func (c *customContext) Err() error {
	// if the parent is canceled, we are racing against propagateCancel so
	// wait for it to complete
	select {
	case <-c.parent.Done():
		<-c.done
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *customContext) Value(key interface{}) interface{} {
	return c.parent.Value(key)
}

func (c *customContext) Deadline() (deadline time.Time, ok bool) {
	return c.parent.Deadline()
}
