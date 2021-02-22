package process_test

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/kevindejong/process"
)

func TestContextNoCancel(t *testing.T) {
	ctx, _ := process.NewContext()
	require.Equal(t, nil, ctx.Err())
	select {
	case <-ctx.Done():
		t.Fatal("context unexpectedly canceld")
	default:
	}
}

func TestContextCancelErr(t *testing.T) {
	ctx, cancel := process.NewContext()
	expectedErr := errors.New("TestContextCancelErr")
	cancel(expectedErr)
	select {
	case <-ctx.Done():
		require.EqualError(t, ctx.Err(), fmt.Sprintf("context canceled: %s", expectedErr.Error()))
	default:
		t.Fatal("context unexpectedly canceld")
	}
}

func TestContextCancelNoErr(t *testing.T) {
	ctx, cancel := process.NewContext()
	cancel(nil)
	select {
	case <-ctx.Done():
		require.EqualError(t, ctx.Err(), "context canceled: <nil>")
		require.Equal(t, nil, errors.Cause(ctx.Err()))
	default:
		t.Fatal("context unexpectedly canceled")
	}
}

func TestContextMultipleCancel(t *testing.T) {
	ctx, cancel := process.NewContext()
	expectedErr := errors.New("TestContextCancelErr")
	cancel(expectedErr)
	require.NotPanics(t, func() {
		cancel(nil)
	})
	select {
	case <-ctx.Done():
		require.EqualError(t, ctx.Err(), fmt.Sprintf("context canceled: %s", expectedErr.Error()))
	default:
		t.Fatal("context not canceld as expected")
	}
}

func TestContextContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	customCtx, _ := process.WithCancel(ctx)
	cancel()
	select {
	case <-customCtx.Done():
		require.EqualError(t, customCtx.Err(), context.Canceled.Error())
	case <-time.After(10 * time.Millisecond): // context cancelation is not instantaneous
		t.Fatal("context not canceled as expected")
	}
}

func TestContextContextCancelThenCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	customCtx, cancelFunc := process.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		cancelFunc(errors.New("unexpected error"))
		close(done)
	}()
	cancel()
	<-done
	select {
	case <-customCtx.Done():
		require.EqualError(t, customCtx.Err(), context.Canceled.Error())
	case <-time.After(10 * time.Millisecond): // context cancelation is not instantaneous
		t.Fatal("context not canceld as expected")
	}
}

func TestContextNoDeadline(t *testing.T) {
	customCtx, _ := process.NewContext()
	ctxDeadline, ok := customCtx.Deadline()
	if ok {
		log.Fatal("expected invalid deadline")
	}
	require.Equal(t, time.Time{}, ctxDeadline)
}

func TestContextContextDeadline(t *testing.T) {
	deadline := time.Time{}.Add(1 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	customCtx, _ := process.WithCancel(ctx)
	ctxDeadline, ok := customCtx.Deadline()
	if !ok {
		log.Fatal("expected valid deadline")
	}
	require.Equal(t, deadline, ctxDeadline)
}

func TestContextNoValues(t *testing.T) {
	customCtx, _ := process.NewContext()
	val := customCtx.Value("abc")
	require.Equal(t, nil, val)
}

type contextTestKey string

func TestContextContextValues(t *testing.T) {
	key := contextTestKey("key")
	value := "value"
	ctx := context.WithValue(context.Background(), key, value)
	customCtx, _ := process.WithCancel(ctx)
	val := customCtx.Value(key)
	require.Equal(t, value, val)
}

func TestContextInner(t *testing.T) {
	customCtx, cancelFunc := process.NewContext()
	defer cancelFunc(nil)
	ctx, cancel := context.WithCancel(customCtx)
	defer cancel()
	expectedErr := errors.New("TestContextInner")
	cancelFunc(expectedErr)
	<-ctx.Done()
	require.EqualError(t, ctx.Err(), fmt.Sprintf("context canceled: %s", expectedErr.Error()))
}
