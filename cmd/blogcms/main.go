package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"blogcms/internal/app"
	"blogcms/internal/auth"
	"blogcms/internal/config"
	"blogcms/internal/postgres"
	"blogcms/internal/version"
	"blogcms/internal/web"
)

func main() {
	fmt.Printf("BlogCMS version: %s\n", version.Version)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := postgres.Open(cfg.DB.DSN, postgres.Options{
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
		PingTimeout:     cfg.DB.PingTimeout,
	})
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer db.Close()

	postRepo := postgres.NewPostRepository(db.DB)
	tagRepo := postgres.NewTagRepository(db.DB)
	userRepo := postgres.NewUserRepository(db.DB)
	settingsRepo := postgres.NewSettingsRepository(db.DB)

	clock := app.RealClock{}
	postSvc := app.NewPostService(postRepo, tagRepo, clock, app.PostServiceOptions{
		MarkdownPoolSize: cfg.App.MarkdownRendererPool,
	})
	authSvc := app.NewAuthService(userRepo, cfg.Security.SessionKey)
	settingsSvc := app.NewSettingsService(settingsRepo)
	if err := settingsSvc.EnsureDefaults(context.Background()); err != nil {
		log.Fatalf("settings error: %v", err)
	}

	renderer, err := web.NewTemplateRenderer()
	if err != nil {
		log.Fatalf("templates error: %v", err)
	}

	services := web.Services{
		Posts:    postSvc,
		Auth:     authSvc,
		Settings: settingsSvc,
		Sessions: auth.NewMemoryStore(),
	}

	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	maxUploadBytes := int64(cfg.Storage.MaxUploadMB) * 1024 * 1024
	srv := web.NewServer(
		cfg.Server.Addr,
		web.ServerOptions{
			CookieSecure:      cfg.Security.CookieSecure,
			ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
			ReadTimeout:       cfg.Server.ReadTimeout,
			WriteTimeout:      cfg.Server.WriteTimeout,
			IdleTimeout:       cfg.Server.IdleTimeout,
			MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
			RequestTimeout:    cfg.Server.RequestTimeout,
			SettingsCacheTTL:  cfg.App.SettingsCacheTTL,
			TagCloudCacheTTL:  cfg.App.TagCloudCacheTTL,
		},
		renderer,
		services,
		app.DefaultBlogTitle,
		app.DefaultBlogAbout,
		app.DefaultBlogFooter,
		cfg.Storage.UploadDir,
		cfg.Storage.PublicBaseURL,
		maxUploadBytes,
	)
	srv.StartBackground(bgCtx)

	go func() {
		log.Printf("listening on %s", cfg.Server.Addr)
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	waitForShutdown(srv, bgCancel, cfg.Server.ShutdownTimeout)
}

func waitForShutdown(srv *web.Server, bgCancel context.CancelFunc, shutdownTimeout time.Duration) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	bgCancel()
	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	_ = srv.Shutdown(ctx)
}
