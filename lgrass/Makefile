VERSION ?= dev
DIST := dist
BIN_DIR := $(HOME)/.local/bin

.PHONY: dist install clean

dist:
	mkdir -p $(DIST)/linux-amd64 $(DIST)/linux-arm64
	GOOS=linux GOARCH=amd64 go build \
	  -ldflags "-X github.com/faizalv/lemongrass/cmd/lgrass/version.Version=$(VERSION)" \
	  -o $(DIST)/linux-amd64/lgrass ./cmd/lgrass
	GOOS=linux GOARCH=arm64 go build \
	  -ldflags "-X github.com/faizalv/lemongrass/cmd/lgrass/version.Version=$(VERSION)" \
	  -o $(DIST)/linux-arm64/lgrass ./cmd/lgrass

# Host-arch build for local dev iteration, installed straight to
# ~/.local/bin. dist cross-compiles both Linux arches for packaging;
# this builds for the host's own GOOS/GOARCH only. Used by the root
# Makefile's dev target.
install:
	mkdir -p $(BIN_DIR)
	go build \
	  -ldflags "-X github.com/faizalv/lemongrass/cmd/lgrass/version.Version=$(VERSION)" \
	  -o $(BIN_DIR)/lgrass ./cmd/lgrass

clean:
	rm -rf $(DIST)
