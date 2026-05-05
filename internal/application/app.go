package application

import (
	"context"
	_ "event_service/docs"
	"event_service/internal/config"
	"event_service/internal/handler"
	"event_service/internal/infrastructure/database"
	"event_service/internal/infrastructure/logger"
	"event_service/internal/middleware"
	"event_service/internal/repository"
	"event_service/internal/server"
	"event_service/internal/services"
	"fmt"
	"sync"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type App struct {
	config   config.Config
	server   *server.HTTPServer
	db       *gorm.DB
	logger   *zap.Logger
	stopOnce sync.Once
}

var (
	newLoggerFn       = logger.NewLogger
	newHTTPServerFn   = server.NewHTTPServer
	connectPostgresFn = database.ConnectPostgres
)

func NewApp(cfg config.Config) (*App, error) {
	zapLogger, err := newLoggerFn(cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	srv := newHTTPServerFn(cfg.Server)

	db, err := connectPostgresFn(cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("failed to init database: %w", err)
	}

	eventRepo := repository.NewEventRepository(db, zapLogger)
	eventService := services.NewEventService(zapLogger, eventRepo)
	eventHandler := handler.NewEventHandler(eventService, zapLogger)

	router := srv.Router()
	router.Use(middleware.CORS())
	router.Use(middleware.RequestID())
	router.Use(middleware.RequestLogger(zapLogger))
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	apiV1 := router.Group("/api/v1")
	eventHandler.RegisterRoutes(apiV1)

	zapLogger.Info("application initialized successfully")

	return &App{
		config: cfg,
		logger: zapLogger,
		server: srv,
		db:     db,
	}, nil
}

func (app *App) Start() error {
	app.logger.Info("starting http server...")
	return app.server.Start()
}

func (app *App) Shutdown(ctx context.Context) error {
	app.logger.Info("shutting down application...")

	if err := app.server.Shutdown(ctx); err != nil {
		app.logger.Error("shutdown server failed", zap.Error(err))
		return err
	}

	app.cleanup()
	app.logger.Info("shutdown application successfully")
	_ = app.logger.Sync()

	return nil
}

func (app *App) ForceShutdown() error {
	app.logger.Warn("forcing application shutdown")

	if err := app.server.Close(); err != nil {
		app.logger.Error("force close server failed", zap.Error(err))
		return err
	}

	app.cleanup()
	app.logger.Info("forced shutdown completed")
	_ = app.logger.Sync()

	return nil
}

func (app *App) cleanup() {
	app.stopOnce.Do(func() {
		sqlDB, err := app.db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}
