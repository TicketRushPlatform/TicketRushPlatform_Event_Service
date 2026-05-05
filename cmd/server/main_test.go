package main

import (
	"context"
	"event_service/internal/config"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

type fakeApp struct {
	startCalls    atomic.Int32
	shutdownCalls atomic.Int32
	started       chan struct{}
	stop          chan struct{}
	shutdownBlock chan struct{}
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

func syscallSigterm() os.Signal {
	// Use os.Interrupt-compatible signal type in tests without importing syscall directly.
	// In production we listen for syscall.SIGTERM; for unit tests, any second signal should trigger force path.
	return os.Kill
}
