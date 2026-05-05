package main

import (
	"context"
	"event_service/internal/application"
	"event_service/internal/config"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// @title Event Service API
// @version 1.0
// @description Event CRUD API for both normal events and movies.
// @BasePath /api/v1
// @schemes http https
func main() {
	os.Exit(run(defaultRunDeps()))
}

type appRunner interface {
	Start() error
	Shutdown(ctx context.Context) error
}

type runDeps struct {
	newConfig       func() config.Config
	newApp          func(cfg config.Config) (appRunner, error)
	notifySignal    func(c chan<- os.Signal, sig ...os.Signal)
	stopSignal      func(c chan<- os.Signal)
	sigCh           chan os.Signal
	shutdownTimeout time.Duration
	logPrintf       func(format string, v ...any)
	logPrintln      func(v ...any)
}

func defaultRunDeps() runDeps {
	return runDeps{
		newConfig:       config.NewConfig,
		newApp:          func(cfg config.Config) (appRunner, error) { return application.NewApp(cfg) },
		notifySignal:    signal.Notify,
		stopSignal:      signal.Stop,
		sigCh:           make(chan os.Signal, 2),
		shutdownTimeout: 15 * time.Second,
		logPrintf:       log.Printf,
		logPrintln:      log.Println,
	}
}

func run(deps runDeps) int {
	cfg := deps.newConfig()
	deps.logPrintln(cfg)

	app, err := deps.newApp(cfg)
	if err != nil {
		panic(err)
	}

	go func() {
		if err := app.Start(); err != nil {
			panic(err)
		}
	}()

	deps.notifySignal(deps.sigCh, os.Interrupt, syscall.SIGTERM)
	defer deps.stopSignal(deps.sigCh)

	sig := <-deps.sigCh
	deps.logPrintf("received signal: %s", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), deps.shutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		if err := app.Shutdown(shutdownCtx); err != nil {
			deps.logPrintf("shutdown err: %v", err)
		}
		close(done)
	}()

	select {
	case <-deps.sigCh:
		deps.logPrintln("force shutdown")
		return 1
	case <-done:
		deps.logPrintln("graceful shutdown complete")
		return 0
	}
}
