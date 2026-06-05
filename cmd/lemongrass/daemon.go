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
	"sync"
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

var projectMarkers = []string{
	"go.mod", "package.json", "pyproject.toml", "Cargo.toml",
	".git", "pom.xml", "build.gradle", "Makefile",
}

var blacklistedPaths = map[string]string{
	"/home":         "System home directory containing all user accounts. Pick a subdirectory instead.",
	"/etc":          "System configuration directory, not a project root.",
	"/lib":          "System library directory, not a project root.",
	"/lib64":        "System library directory, not a project root.",
	"/lib32":        "System library directory, not a project root.",
	"/usr":          "System directory, not a project root.",
	"/var":          "System data directory, not a project root.",
	"/tmp":          "Temporary files directory, not a project root.",
	"/opt":          "Software installation directory, not typically a project root.",
	"/System":       "macOS system directory, not a project root.",
	"/Library":      "macOS library directory, not a project root.",
	"/Applications": "macOS applications directory, not a project root.",
}

func blacklistWarning(path string) string {
	clean := filepath.Clean(path)
	if reason, ok := blacklistedPaths[clean]; ok {
		return reason
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if clean == filepath.Clean(home) {
		return fmt.Sprintf("Your home directory. Projects typically live in subdirectories like %s/myproject, not the home root itself.", home)
	}
	return ""
}

func validateProjectDir(path string) (bool, []string) {
	if reason := blacklistWarning(path); reason != "" {
		return false, []string{reason}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return true, nil
	}

	hasRootMarker := false
	for _, marker := range projectMarkers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			hasRootMarker = true
			break
		}
	}

	subProjectCount := 0
	dirCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirCount++
		for _, marker := range projectMarkers {
			if _, err := os.Stat(filepath.Join(path, entry.Name(), marker)); err == nil {
				subProjectCount++
				break
			}
		}
	}

	var warnings []string
	if subProjectCount >= 3 {
		warnings = append(warnings, fmt.Sprintf("Contains %d subdirectories that each appear to be separate projects. This looks like a container directory rather than a single project root.", subProjectCount))
	}
	if !hasRootMarker && dirCount >= 10 {
		warnings = append(warnings, fmt.Sprintf("Contains %d subdirectories with no recognizable project marker. Indexing this could produce an unusually large and noisy semantic map.", dirCount))
	}
	return len(warnings) == 0, warnings
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

	concurrency := cfg.FsConcurrency
	if concurrency <= 0 {
		concurrency = 8
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		visited = make(map[string]struct{})
		sem     = make(chan struct{}, concurrency)
		bw      = bufio.NewWriter(w)
	)

	var walk func(path string)
	walk = func(path string) {
		defer wg.Done()

		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return
		}

		mu.Lock()
		_, seen := visited[real]
		if !seen {
			visited[real] = struct{}{}
		}
		mu.Unlock()

		if seen {
			return
		}

		sem <- struct{}{}
		entries, err := os.ReadDir(path)
		<-sem

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

			mu.Lock()
			fmt.Fprintln(bw, full)
			mu.Unlock()

			wg.Add(1)
			go walk(full)
		}
	}

	entries, err := os.ReadDir("/")
	if err == nil {
		for _, entry := range entries {
			if excluded[entry.Name()] {
				continue
			}
			full := filepath.Join("/", entry.Name())
			info, err := os.Stat(full)
			if err != nil || !info.IsDir() {
				continue
			}

			mu.Lock()
			fmt.Fprintln(bw, full)
			mu.Unlock()

			wg.Add(1)
			go walk(full)
		}
	}

	wg.Wait()
	bw.Flush()
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

	case "VALIDATE":
		if !scanner.Scan() {
			return
		}
		ok, warnings := validateProjectDir(scanner.Text())
		w := bufio.NewWriter(conn)
		if ok {
			fmt.Fprintln(w, "OK")
		} else {
			fmt.Fprintln(w, "WARN")
			for _, warn := range warnings {
				fmt.Fprintln(w, warn)
			}
		}
		fmt.Fprintln(w, "END")
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
