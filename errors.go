package process

import (
	"fmt"
	"runtime"
	"strings"
)

type Frame struct {
	PC   uintptr
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
	// find the frame first frame excluding the current package
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
		PC:   pc,
		Name: name,
		File: file,
		Line: line,
	}
}

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

type errContextFrame struct {
	cause error
	frame Frame
}

func wrapContextFrame(err error) error {
	frame := externalFrame()
	if frame != nil {
		return &errContextFrame{cause: err, frame: *frame}
	}
	return err
}

func withContextFrame(err error, frame *Frame) error {
	if frame != nil {
		return &errContextFrame{cause: err, frame: *frame}
	}
	return err
}

func (e *errContextFrame) Error() string {
	return e.cause.Error()
}

func (e *errContextFrame) Cause() error {
	return e.cause
}

func (e *errContextFrame) Unwrap() error {
	return e.cause
}

func (e *errContextFrame) Trace() []Frame {
	if cause, ok := e.cause.(*errContextFrame); ok {
		return append([]Frame{e.frame}, cause.Trace()...)
	}
	return []Frame{}
}

func (e *errContextFrame) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintln(s, e.frame.Name)
			fmt.Fprintf(s, "\t%s:%d\n", e.frame.File, e.frame.Line)
			fmt.Fprintf(s, "%+v", e.cause)
			return
		}
		fallthrough
	case 's':
		fmt.Fprintf(s, "%s", e.cause)
	case 'q':
		fmt.Fprintf(s, "%q", e.cause)
	}
}
