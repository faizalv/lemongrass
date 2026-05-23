package config

import (
	"bytes"
	"path/filepath"
	"text/template"
)

var tmpl = template.Must(template.New("compose").Parse(`services:
  lg-server:
    container_name: lg-server
    image: lemongrass-server:latest
    ports:
      - "{{.Port}}:9966"
    volumes:
      - {{.LGDir}}:/root/.lemongrass
      - {{.LGDir}}/logs:/var/log/lemongrass
      - /var/run/docker.sock:/var/run/docker.sock
      - {{.BinPath}}:/usr/local/bin/lemongrass:ro
{{- range .ProjectMounts}}
      - {{.HostPath}}:/projects/{{.Alias}}:rw
{{- end}}
    depends_on:
      lg-postgres:
        condition: service_healthy
      lg-redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-sf", "http://localhost:9966/api/health"]
      interval: 10s
      timeout: 5s
      retries: 6
      start_period: 15s
    restart: unless-stopped

  lg-runner:
    container_name: lg-runner
    image: lemongrass-runner:latest
    volumes:
      - {{.LGDir}}/claude:/home/lg/.lemongrass/claude
      - {{.LGDir}}/logs:/var/log/lemongrass
    environment:
      - CLAUDE_CONFIG_DIR=/home/lg/.lemongrass/claude
    restart: unless-stopped

  lg-embed:
    container_name: lg-embed
    image: lemongrass-embed:latest
    healthcheck:
      test: ["CMD", "python", "-c", "import urllib.request; urllib.request.urlopen('http://localhost:8080/health')"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 30s
    restart: unless-stopped

  lg-postgres:
    container_name: lg-postgres
    image: pgvector/pgvector:pg16
    volumes:
      - {{.LGDir}}/postgres:/var/lib/postgresql/data
    environment:
      - POSTGRES_USER=lemongrass
      - POSTGRES_PASSWORD=lemongrass
      - POSTGRES_DB=lemongrass
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U lemongrass"]
      interval: 5s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  lg-redis:
    container_name: lg-redis
    image: redis:7
    volumes:
      - {{.LGDir}}/redis:/data
    command: redis-server --save 60 1
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5
    restart: unless-stopped

networks:
  default:
    name: lemongrass
    driver: bridge
`))

type projectMount struct {
	HostPath string
	Alias    string
}

type composeData struct {
	Config
	LGDir         string
	ProjectMounts []projectMount
}

func GenerateCompose(cfg Config, projectPaths []string) []byte {
	mounts := make([]projectMount, len(projectPaths))
	for i, p := range projectPaths {
		mounts[i] = projectMount{HostPath: p, Alias: filepath.Base(p)}
	}
	lgDir := filepath.Join(cfg.HomeDir, ".lemongrass")
	if cfg.HomeDir == "" {
		lgDir = Dir()
	}
	var buf bytes.Buffer
	tmpl.Execute(&buf, composeData{Config: cfg, LGDir: lgDir, ProjectMounts: mounts})
	return buf.Bytes()
}
