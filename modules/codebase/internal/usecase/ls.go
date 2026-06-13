package usecase

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ls(projectDir, args string) string {
	target := projectDir
	if args != "" {
		target = filepath.Join(projectDir, filepath.Clean(args))
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return "error: " + err.Error()
	}
	if len(entries) == 0 {
		return "empty directory"
	}
	var sb strings.Builder
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if e.IsDir() {
			dirs, files := countDirContents(filepath.Join(target, e.Name()))
			fmt.Fprintf(&sb, "%s/  -- %d dirs; %d files\n", e.Name(), dirs, files)
		} else {
			data, err := os.ReadFile(filepath.Join(target, e.Name()))
			if err != nil {
				fmt.Fprintf(&sb, "%s\n", e.Name())
				continue
			}
			lines := bytes.Count(data, []byte("\n"))
			if len(data) > 0 && data[len(data)-1] != '\n' {
				lines++
			}
			fmt.Fprintf(&sb, "%s  -- %s lines; %s chars\n", e.Name(), formatCount(lines), formatCount(len(data)))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func countDirContents(dir string) (dirs, files int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	for _, e := range entries {
		if e.IsDir() {
			dirs++
			d, f := countDirContents(filepath.Join(dir, e.Name()))
			dirs += d
			files += f
		} else {
			files++
		}
	}
	return
}

func formatCount(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	return strconv.Itoa(n/1000) + "k"
}
