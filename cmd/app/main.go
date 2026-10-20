package main

import (
	"context"
	"database/sql"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	"github.com/testTelegramApp/expenses-bot/internal/agent"
	"github.com/testTelegramApp/expenses-bot/internal/config"
	"github.com/testTelegramApp/expenses-bot/internal/repository"
	"github.com/testTelegramApp/expenses-bot/internal/service"
	httpapi "github.com/testTelegramApp/expenses-bot/internal/transport/http"
	"github.com/testTelegramApp/expenses-bot/internal/transport/telegram"
)

//go:embed all:static
var staticFS embed.FS

func main() {
	cfgPath := flag.String("config", "configs/config.yaml", "path to config yaml")
	flag.Parse()

	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer log.Sync() //nolint:errcheck

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal("config", zap.Error(err))
	}

	db, err := sqlx.Connect("pgx", cfg.Database.DSN)
	if err != nil {
		log.Fatal("db", zap.Error(err))
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err := runMigrations(db.DB); err != nil {
		log.Fatal("migrate", zap.Error(err))
	}

	loc := cfg.Location()
	userRepo := repository.NewUserRepository(db)
	expRepo := repository.NewExpenseRepository(db)
	userSvc := service.NewUserService(userRepo)
	expSvc := service.NewExpenseService(expRepo, userRepo, loc)

	var llm agent.LLMClient
	if cfg.LLM.Mock || cfg.LLM.APIKey == "" {
		log.Info("using mock LLM")
		llm = agent.NewMockLLM()
	} else {
		timeout := time.Duration(cfg.LLM.TimeoutSec) * time.Second
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		llm = agent.NewOpenAIClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, timeout)
	}
	ag := agent.New(llm, expSvc, userSvc, cfg.Currency, cfg.LLM, loc, log)
	defer ag.Close()

	r := chi.NewRouter()
	r.Use(httpapi.RequestID)
	r.Use(httpapi.Logger(log))
	r.Use(httpapi.CORS)

	r.Get("/healthz", httpapi.Healthz)

	api := &httpapi.API{Expenses: expSvc, Loc: loc}
	r.Route("/api/v1", func(ar chi.Router) {
		ar.Use(httpapi.BasicAuth(cfg.Admin.User, cfg.Admin.Pass))
		api.Routes(ar)
	})

	var tgHandler *telegram.Handler
	if cfg.Telegram.Token != "" {
		th, err := telegram.New(cfg.Telegram.Token, userSvc, ag, log, cfg.Telegram.WebhookPath, cfg.Telegram.WebhookURL)
		if err != nil {
			log.Fatal("telegram", zap.Error(err))
		}
		tgHandler = th
		path := cfg.Telegram.WebhookPath
		if path == "" {
			path = "/telegram/webhook"
		}
		if th.Mode() == "webhook" {
			r.Post(path, th.WebhookHandler().ServeHTTP)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			if err := th.SetWebhook(ctx, cfg.Telegram.WebhookURL); err != nil {
				log.Warn("set webhook", zap.Error(err))
			} else {
				log.Info("webhook set", zap.String("url", cfg.Telegram.WebhookURL))
			}
			cancel()
		}
	} else {
		log.Warn("TELEGRAM_TOKEN empty — bot disabled")
	}

	mountStatic(r)

	srv := &http.Server{Addr: cfg.Server.Addr, Handler: r}
	go func() {
		log.Info("listening", zap.String("addr", cfg.Server.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("serve", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if tgHandler != nil {
		tgHandler.Close()
	}
	_ = srv.Shutdown(ctx)
	log.Info("shutdown complete")
}

func runMigrations(db *sql.DB) error {
	// Prefer filesystem migrations next to binary / working dir.
	dirs := []string{"migrations", "/app/migrations"}
	for _, d := range dirs {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			if err := goose.SetDialect("postgres"); err != nil {
				return err
			}
			return goose.Up(db, d)
		}
	}
	return fmt.Errorf("migrations directory not found")
}

func mountStatic(r chi.Router) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return
	}
	// If placeholder only (no index), also try disk web/dist for local run.
	fileServer := http.FileServer(http.FS(sub))
	r.Handle("/*", spaHandler(fileServer, sub))
}

type spaFileSystem interface {
	Open(name string) (fs.File, error)
}

func spaHandler(fileServer http.Handler, fsys spaFileSystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = path[1:]
		}
		if f, err := fsys.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// fallback to disk for local `make run` after vite build
		disk := filepath.Join("web", "dist", path)
		if _, err := os.Stat(disk); err == nil {
			http.ServeFile(w, r, disk)
			return
		}
		indexDisk := filepath.Join("web", "dist", "index.html")
		if _, err := os.Stat(indexDisk); err == nil {
			http.ServeFile(w, r, indexDisk)
			return
		}
		if f, err := fsys.Open("index.html"); err == nil {
			_ = f.Close()
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		http.NotFound(w, r)
	})
}
