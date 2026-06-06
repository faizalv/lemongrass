package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Version        string   `json:"version"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	PostgresDSN    string   `json:"postgres_dsn"`
	LogLevel       string   `json:"log_level"`
	FsExtraExclude []string `json:"fs_extra_exclude"`
	FsConcurrency  int      `json:"fs_concurrency"`
	Languages      []string `json:"languages"`
	HomeDir        string   `json:"home_dir"`
	BinPath        string   `json:"bin_path"`
}

var defaults = Config{
	Version:        "0.1.0",
	Host:           "localhost",
	Port:           9966,
	PostgresDSN:    "postgres://lemongrass:lemongrass@lg-postgres:5432/lemongrass?sslmode=disable",
	LogLevel:       "info",
	FsExtraExclude: []string{},
	FsConcurrency:  8,
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lemongrass")
}

func DetectBinPath() string {
	home, _ := os.UserHomeDir()
	local := filepath.Join(home, ".local", "bin", "lemongrass")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	return "/usr/local/bin/lemongrass"
}

func configPath() string {
	return filepath.Join(Dir(), "config.json")
}

func LoadOrDefault() Config {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return defaults
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaults
	}
	if cfg.FsConcurrency <= 0 {
		cfg.FsConcurrency = defaults.FsConcurrency
	}
	return cfg
}

func Save(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0644)
}

func EnsureScaffold() {
	dirs := []string{
		Dir(),
		filepath.Join(Dir(), "claude"),
		filepath.Join(Dir(), "projects"),
		filepath.Join(Dir(), "postgres"),
		filepath.Join(Dir(), "logs"),
		filepath.Join(Dir(), "workspaces"),
		filepath.Join(Dir(), "grammars"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0755)
	}

	cfgPath := configPath()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		data, _ := json.MarshalIndent(defaults, "", "  ")
		os.WriteFile(cfgPath, data, 0644)
	}
}
