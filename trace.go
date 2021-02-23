package process

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Frame struct {
	errors.Frame
	Name string
	File string
	Line int
}

func externalFrame() *Frame {
	// get the call stack excluding the runtime call
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(1, pcs[:])
	st := pcs[0:n]
	// get the current package name
	pkgFrame := frameDetails(st[0])
	pkg := pkgFrame.Name[0:strings.LastIndex(pkgFrame.Name, ".")]
	// find the frame first frame excluding the current package and go runtime
	for _, s := range st[1:] {
		fr := frameDetails(s)
		if !strings.HasPrefix(fr.Name, pkg+".") && !strings.HasPrefix(fr.Name, "runtime.") {
			return &fr
		}
	}
	return nil
}

func frameDetails(pc uintptr) Frame {
	fn := runtime.FuncForPC(pc - 1)
	name := fn.Name()
	file, line := fn.FileLine(pc - 1)
	return Frame{
		Frame: errors.Frame(pc),
		Name:  name,
		File:  file,
		Line:  line,
	}
}

type traceError struct {
	cause error
	frame Frame
}

func WrapTrace(err error) error {
	if err == nil {
		return nil
	}
	frame := externalFrame()
	if frame == nil {
		return err
	}
	return &traceError{
		cause: err,
		frame: *frame,
	}
}

func (t *traceError) Error() string {
	return t.cause.Error()
}

func (t *traceError) Cause() error {
	return t.cause
}

func (t *traceError) Unwrap() error {
	return t.cause
}

func (t *traceError) Trace() []Frame {
	err := t.cause
	for err != nil {
		if trace, ok := err.(*traceError); ok {
			return append([]Frame{t.frame}, trace.Trace()...)
		}
		if errors.Unwrap(err) != nil {
			err = errors.Unwrap(err)
			continue
		}
		if errors.Cause(err) != err {
			err = errors.Cause(err)
			continue
		}
		break
	}
	return []Frame{t.frame}
}

func (t *traceError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintln(s, t.frame.Name)
			fmt.Fprintf(s, "\t%s:%d\n", t.frame.File, t.frame.Line)
			fmt.Fprintf(s, "%+v", t.cause)
			return
		}
		fallthrough
	case 's':
		fmt.Fprintf(s, "%s", t.cause)
	case 'q':
		fmt.Fprintf(s, "%q", t.cause)
	}
}

type traceContext struct {
	parent context.Context
	frame  Frame
}

func WithTrace(ctx context.Context) context.Context {
	frame := externalFrame()
	if frame == nil {
		return ctx
	}
	t := &traceContext{
		parent: ctx,
		frame:  *frame,
	}
	return t
}

func (t *traceContext) Done() <-chan struct{} {
	return t.parent.Done()
}

func (t *traceContext) Err() error {
	if t.parent.Err() == nil {
		return nil
	}
	return &traceError{cause: WrapTrace(t.parent.Err()), frame: t.frame}
}

func (t *traceContext) Deadline() (time.Time, bool) {
	return t.parent.Deadline()
}

func (t *traceContext) Value(key interface{}) interface{} {
	return t.parent.Value(key)
}
