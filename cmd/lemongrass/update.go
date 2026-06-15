package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/faizalv/lemongrass/cmd/lemongrass/version"
)

func cmdUpdate() {
	resp, err := http.Get("https://api.github.com/repos/faizalv/lemongrass/releases/latest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: could not reach GitHub: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Fprintf(os.Stderr, "update: could not parse release: %v\n", err)
		os.Exit(1)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	if latest == version.Version {
		fmt.Printf("Already on v%s. Nothing to do.\n", version.Version)
		return
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goarch != "amd64" && goarch != "arm64" {
		fmt.Fprintf(os.Stderr, "update: unsupported architecture: %s\n", goarch)
		os.Exit(1)
	}

	binaryName := fmt.Sprintf("lemongrass-%s-%s", goos, goarch)
	baseURL := fmt.Sprintf("https://github.com/faizalv/lemongrass/releases/download/v%s", latest)

	fmt.Printf("Updating to v%s...\n", latest)

	tmp, err := os.CreateTemp("", "lemongrass-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: %v\n", err)
		os.Exit(1)
	}
	tmpName := tmp.Name()

	binResp, err := http.Get(fmt.Sprintf("%s/%s", baseURL, binaryName))
	if err != nil {
		tmp.Close()
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "update: download failed: %v\n", err)
		os.Exit(1)
	}
	defer binResp.Body.Close()
	if binResp.StatusCode != 200 {
		tmp.Close()
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "update: download failed: HTTP %d\n", binResp.StatusCode)
		os.Exit(1)
	}

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), binResp.Body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "update: write failed: %v\n", err)
		os.Exit(1)
	}
	tmp.Close()
	actual := hex.EncodeToString(h.Sum(nil))

	csResp, err := http.Get(fmt.Sprintf("%s/checksums.txt", baseURL))
	if err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "update: could not fetch checksums: %v\n", err)
		os.Exit(1)
	}
	defer csResp.Body.Close()
	csData, _ := io.ReadAll(csResp.Body)

	expected := ""
	for _, line := range strings.Split(string(csData), "\n") {
		if strings.Contains(line, binaryName) {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				expected = parts[0]
			}
			break
		}
	}
	if expected == "" {
		os.Remove(tmpName)
		fmt.Fprintln(os.Stderr, "update: binary not found in checksums.txt")
		os.Exit(1)
	}
	if actual != expected {
		os.Remove(tmpName)
		fmt.Fprintln(os.Stderr, "update: checksum mismatch")
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "update: could not locate binary: %v\n", err)
		os.Exit(1)
	}
	if err := os.Chmod(tmpName, 0755); err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "update: chmod failed: %v\n", err)
		os.Exit(1)
	}
	if err := os.Rename(tmpName, exe); err != nil {
		fmt.Fprintf(os.Stderr, "update: could not replace binary (permission denied)\n")
		fmt.Fprintf(os.Stderr, "Run: sudo mv %s %s\n", tmpName, exe)
		os.Exit(1)
	}

	fmt.Printf("Updated to v%s. Restart your shell for completion changes.\n", latest)
}
