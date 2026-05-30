package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/faizalv/lemongrass/config"
)

func cmdStartDaemon() {
	sockPath := filepath.Join(config.Dir(), "lemongrass.sock")
	pidPath := filepath.Join(config.Dir(), "lg-daemon.pid")

	os.Remove(sockPath)

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lemongrass-daemon: %v\n", err)
		os.Exit(1)
	}

	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		l.Close()
	}()

	defer os.Remove(sockPath)
	defer os.Remove(pidPath)

	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go serveConn(conn)
	}
}

var defaultFsExclusions = map[string]bool{
	"proc": true, "sys": true, "dev": true,
	"run": true, "tmp": true, "boot": true, "lost+found": true,
	"node_modules": true, ".git": true, "vendor": true,
	"__pycache__": true, ".venv": true, "venv": true,
	"dist": true, "build": true, "target": true,
	".next": true, ".nuxt": true, "coverage": true,
}

func walkFS(w io.Writer) {
	cfg := config.LoadOrDefault()
	excluded := make(map[string]bool, len(defaultFsExclusions)+len(cfg.FsExtraExclude))
	for k := range defaultFsExclusions {
		excluded[k] = true
	}
	for _, e := range cfg.FsExtraExclude {
		excluded[e] = true
	}
	visited := make(map[string]struct{})
	walkDir("/", excluded, visited, w)
}

func walkDir(path string, excluded map[string]bool, visited map[string]struct{}, w io.Writer) {
	reald, err := filepath.EvalSymlinks(path)
	if err != nil {
		return
	}
	if _, seen := visited[reald]; seen {
		return
	}
	visited[reald] = struct{}{}

	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if excluded[entry.Name()] {
			continue
		}
		full := filepath.Join(path, entry.Name())
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		fmt.Fprintln(w, full)
		walkDir(full, excluded, visited, w)
	}
}

func serveConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}

	switch scanner.Text() {
	case "BROWSE":
		w := bufio.NewWriter(conn)
		walkFS(w)
		w.Flush()

	case "REMOUNT":
		var paths []string
		for scanner.Scan() {
			if p := scanner.Text(); p != "" {
				paths = append(paths, p)
			}
		}
		if len(paths) > 0 {
			go exec.Command("lemongrass", append([]string{"remount"}, paths...)...).Start()
		}
	}
}
