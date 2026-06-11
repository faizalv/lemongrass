package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
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
	lgdebug "github.com/faizalv/lemongrass/modules/debug"
	lgfs "github.com/faizalv/lemongrass/modules/fs"
	lglg "github.com/faizalv/lemongrass/modules/lg"
	lgpty "github.com/faizalv/lemongrass/modules/pty"
	lgrecon "github.com/faizalv/lemongrass/modules/recon"
	lgworkspace "github.com/faizalv/lemongrass/modules/workspace"
	lgui "github.com/faizalv/lemongrass/ui"
	"github.com/gin-gonic/gin"
)

const logDir = "/var/log/lemongrass"

func main() {
	cfg := config.LoadOrDefault()

	if lf := setupLogger(logDir); lf != nil {
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

	r := gin.Default()

	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": cfg.Version})
	})

	api.POST("/convert", func(c *gin.Context) {
		var req struct {
			Content string `json:"content"`
			Ext     string `json:"ext"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		body, _ := json.Marshal(req)
		resp, err := http.Post("http://lg-embed:8080/convert-content", "application/json", bytes.NewReader(body))
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "embed unavailable: " + err.Error()})
			return
		}
		defer resp.Body.Close()
		var result struct {
			Markdown string `json:"markdown"`
			Detail   string `json:"detail"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		if resp.StatusCode != http.StatusOK {
			c.JSON(resp.StatusCode, gin.H{"error": result.Detail})
			return
		}
		c.JSON(http.StatusOK, gin.H{"markdown": result.Markdown})
	})

	ptyMod := &lgpty.Pty{}
	ptyMod.LoadMe(cfg, db)
	defer ptyMod.Close()
	ptyMod.StartHTTPRouter(api)

	fsModule := &lgfs.Fs{}
	fsModule.LoadMe(cfg, db)
	fsModule.StartHTTPRouter(api)

	reconModule := &lgrecon.Recon{}
	reconModule.LoadMe(cfg, db)
	defer reconModule.Close()
	reconModule.StartHTTPRouter(api)

	lgMod := &lglg.Lg{ReconClient: reconModule.Client()}
	lgMod.LoadMe(cfg, db)
	lgMod.SetUsageProvider(ptyMod.Client())
	lgMod.StartUsageScheduler(context.Background())
	lgMod.StartHTTPRouter(api)

	debugMod := &lgdebug.Debug{PtyClient: ptyMod.Client(), SessionRegistrar: lgMod.SessionManager()}
	debugMod.LoadMe(cfg, db)
	debugMod.StartHTTPRouter(api)

	workspaceModule := &lgworkspace.Workspace{PtyClient: ptyMod.Client()}
	workspaceModule.LoadMe(cfg, db)
	lgMod.SetWorkspaceTaskClient(workspaceModule.TaskClient())
	workspaceModule.SetLgSession(lgMod.SessionManager())
	workspaceModule.StartHTTPRouter(api)

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
			if _, err := os.Stat(filepath.Join(dir, ".lemongrass")); err == nil {
				reconModule.Activate(p.ID)
				log.Printf("startup activate: project %d (headless)", p.ID)
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

	socketPath := filepath.Join("/root/.lemongrass", "lg.sock")
	os.Remove(socketPath)
	if unixLn, err := net.Listen("unix", socketPath); err != nil {
		log.Printf("unix socket unavailable: %v", err)
	} else {
		os.Chmod(socketPath, 0666)
		go http.Serve(unixLn, r)
		defer os.Remove(socketPath)
		log.Printf("unix socket: %s", socketPath)
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

func setupLogger(dir string) io.Closer {
	w := infra.NewDailyRotateWriter(dir, "server", 7)
	log.SetOutput(io.MultiWriter(os.Stderr, w))
	log.SetFlags(log.LstdFlags)
	return w
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
