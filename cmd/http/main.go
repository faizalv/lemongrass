package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/infra"
	"github.com/faizalv/lemongrass/migrations"
	lgfs "github.com/faizalv/lemongrass/modules/fs"
	lglg "github.com/faizalv/lemongrass/modules/lg"
	lgpty "github.com/faizalv/lemongrass/modules/pty"
	lgrecon "github.com/faizalv/lemongrass/modules/recon"
	lgui "github.com/faizalv/lemongrass/ui"
	"github.com/gin-gonic/gin"
)

const serverLogPath = "/var/log/lemongrass/server.log"

func main() {
	cfg := config.LoadOrDefault()

	if lf := setupLogger(serverLogPath); lf != nil {
		defer lf.Close()
	}

	db, err := infra.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	if err := infra.RunMigrations(cfg.PostgresDSN, migrations.Files); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	log.Println("migrations: ok")

	rds := infra.NewRedis(cfg.RedisAddr)
	defer rds.Close()

	if err := infra.PingRedis(context.Background(), rds); err != nil {
		log.Printf("redis ping failed: %v", err)
	} else {
		log.Println("redis: ok")
	}

	r := gin.Default()

	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": cfg.Version})
	})

	ptyMod := &lgpty.Pty{}
	ptyMod.LoadMe(cfg, db, rds)
	defer ptyMod.Close()
	ptyMod.StartHTTPRouter(api)

	fsModule := &lgfs.Fs{}
	fsModule.LoadMe(cfg, db, rds)
	fsModule.StartHTTPRouter(api)

	lgMod := &lglg.Lg{PtyClient: ptyMod.Client()}
	lgMod.LoadMe(cfg, db, rds)
	lgMod.StartHTTPRouter(api)

	reconModule := &lgrecon.Recon{}
	reconModule.LoadMe(cfg, db, rds)
	reconModule.StartHTTPRouter(api)

	fsModule.Startup(context.Background())
	log.Println("startup sanity check: ok")

	startupCtx := context.Background()
	projects, err := fsModule.ListProjects()
	if err != nil {
		log.Printf("startup mapping: could not list projects: %v", err)
	} else {
		for _, p := range projects {
			if p.Status == "removed" {
				continue
			}
			dir := "/projects/" + filepath.Base(p.Path)
			if err := reconModule.MapIfNeeded(startupCtx, p.ID, dir); err != nil {
				log.Printf("startup mapping: project %d (%s): %v", p.ID, p.Path, err)
			} else {
				log.Printf("startup mapping: project %d ok", p.ID)
			}
		}
	}

	distFS, err := fs.Sub(lgui.Dist, "dist")
	if err != nil {
		log.Fatalf("ui dist: %v", err)
	}
	r.NoRoute(spaHandler(distFS))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reconModule.StartScheduler(ctx)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	log.Printf("server running on :%d", cfg.Port)
	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Println("server stopped")
}

func setupLogger(logPath string) *os.File {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("could not open log file %s: %v", logPath, err)
		return nil
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetFlags(log.LstdFlags)
	return f
}

func spaHandler(distFS fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(distFS))
	return func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := distFS.Open(path); err != nil {
			c.Request.URL.Path = "/"
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}
