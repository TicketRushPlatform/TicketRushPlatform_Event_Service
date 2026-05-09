package main

import (
	"context"
	"errors"
	"event_service/internal/config"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

type fakeApp struct {
	startCalls        atomic.Int32
	shutdownCalls     atomic.Int32
	started           chan struct{}
	stop              chan struct{}
	shutdownBlock     chan struct{}
	shutdownReturnErr error
}

func newFakeApp() *fakeApp {
	return &fakeApp{
		started: make(chan struct{}),
		stop:    make(chan struct{}),
	}
}

func (a *fakeApp) Start() error {
	a.startCalls.Add(1)
	select {
	case <-a.started:
		// already closed
	default:
		close(a.started)
	}
	<-a.stop
	return nil
}

func (a *fakeApp) Shutdown(ctx context.Context) error {
	a.shutdownCalls.Add(1)
	select {
	case <-a.stop:
		// already closed
	default:
		close(a.stop)
	}
	if a.shutdownReturnErr != nil {
		return a.shutdownReturnErr
	}
	if a.shutdownBlock != nil {
		select {
		case <-a.shutdownBlock:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func TestRun_GracefulShutdown(t *testing.T) {
	app := newFakeApp()
	sigCh := make(chan os.Signal, 2)

	deps := runDeps{
		newConfig:       func() config.Config { return config.Config{} },
		newApp:          func(cfg config.Config) (appRunner, error) { return app, nil },
		notifySignal:    func(c chan<- os.Signal, sig ...os.Signal) {},
		stopSignal:      func(c chan<- os.Signal) {},
		sigCh:           sigCh,
		shutdownTimeout: 200 * time.Millisecond,
		logPrintf:       func(string, ...any) {},
		logPrintln:      func(...any) {},
	}

	codeCh := make(chan int, 1)
	go func() { codeCh <- run(deps) }()

	select {
	case <-app.started:
		// ok
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for app start")
	}

	sigCh <- os.Interrupt

	select {
	case code := <-codeCh:
		if code != 0 {
			t.Fatalf("exit=%d want=0", code)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for run to return")
	}

	if app.shutdownCalls.Load() != 1 {
		t.Fatalf("shutdownCalls=%d want=1", app.shutdownCalls.Load())
	}
}

func TestRun_ForceShutdownOnSecondSignal(t *testing.T) {
	app := newFakeApp()
	app.shutdownBlock = make(chan struct{})
	sigCh := make(chan os.Signal, 2)

	deps := runDeps{
		newConfig:       func() config.Config { return config.Config{} },
		newApp:          func(cfg config.Config) (appRunner, error) { return app, nil },
		notifySignal:    func(c chan<- os.Signal, sig ...os.Signal) {},
		stopSignal:      func(c chan<- os.Signal) {},
		sigCh:           sigCh,
		shutdownTimeout: 200 * time.Millisecond,
		logPrintf:       func(string, ...any) {},
		logPrintln:      func(...any) {},
	}

	codeCh := make(chan int, 1)
	go func() { codeCh <- run(deps) }()

	select {
	case <-app.started:
		// ok
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for app start")
	}

	sigCh <- os.Interrupt

	// wait until shutdown is invoked (or time out)
	deadline := time.Now().Add(time.Second)
	for app.shutdownCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if app.shutdownCalls.Load() == 0 {
		close(app.shutdownBlock)
		t.Fatalf("expected shutdown to be called")
	}

	sigCh <- syscallSigterm()

	select {
	case code := <-codeCh:
		if code != 1 {
			close(app.shutdownBlock)
			t.Fatalf("exit=%d want=1", code)
		}
	case <-time.After(time.Second):
		close(app.shutdownBlock)
		t.Fatalf("timeout waiting for force shutdown")
	}

	close(app.shutdownBlock)
}

func TestDefaultRunDeps(t *testing.T) {
	d := defaultRunDeps()
	if d.newConfig == nil || d.newApp == nil || d.notifySignal == nil || d.stopSignal == nil ||
		d.sigCh == nil || d.logPrintf == nil || d.logPrintln == nil {
		t.Fatalf("unexpected zero deps")
	}
	if d.shutdownTimeout != 15*time.Second {
		t.Fatalf("shutdownTimeout=%v", d.shutdownTimeout)
	}
	cfg := d.newConfig()
	_ = cfg
}

func TestRun_LogsShutdownError(t *testing.T) {
	app := newFakeApp()
	app.shutdownReturnErr = errors.New("shutdown failed")

	sigCh := make(chan os.Signal, 2)
	var sawShutdownLog bool
	deps := runDeps{
		newConfig: func() config.Config { return config.Config{} },
		newApp:    func(cfg config.Config) (appRunner, error) { return app, nil },
		notifySignal: func(c chan<- os.Signal, sig ...os.Signal) {
		},
		stopSignal:      func(c chan<- os.Signal) {},
		sigCh:           sigCh,
		shutdownTimeout: 200 * time.Millisecond,
		logPrintf: func(format string, v ...any) {
			if len(v) > 0 {
				for _, arg := range v {
					if err, ok := arg.(error); ok && err != nil && err.Error() == "shutdown failed" {
						sawShutdownLog = true
					}
				}
			}
			_, _ = format, v
		},
		logPrintln: func(...any) {},
	}

	codeCh := make(chan int, 1)
	go func() { codeCh <- run(deps) }()

	select {
	case <-app.started:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for start")
	}

	sigCh <- os.Interrupt

	select {
	case code := <-codeCh:
		if code != 0 {
			t.Fatalf("exit=%d want=0", code)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}

	if !sawShutdownLog {
		t.Fatalf("expected shutdown error to be logged")
	}
	if app.shutdownCalls.Load() != 1 {
		t.Fatalf("shutdownCalls=%d", app.shutdownCalls.Load())
	}
}

func syscallSigterm() os.Signal {
	// Use os.Interrupt-compatible signal type in tests without importing syscall directly.
	// In production we listen for syscall.SIGTERM; for unit tests, any second signal should trigger force path.
	return os.Kill
}
