VERSION ?= dev
DIST := dist
BIN_DIR := $(HOME)/.local/bin
D_BIN_DIR := $(HOME)/.lemongrass/bin
LDFLAGS := -ldflags "-X github.com/faizalv/lemongrass/cmd/lgrass/version.Version=$(VERSION)"

.PHONY: dist dist-linux dist-darwin install verify clean

dist: dist-linux dist-darwin

dist-linux:
	mkdir -p $(DIST)/linux-amd64 $(DIST)/linux-arm64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/linux-amd64/lgrass ./cmd/lgrass
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/linux-amd64/lgrassd ./cmd/lgrassd
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/linux-arm64/lgrass ./cmd/lgrass
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/linux-arm64/lgrassd ./cmd/lgrassd

dist-darwin:
	mkdir -p $(DIST)/darwin-amd64 $(DIST)/darwin-arm64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/darwin-amd64/lgrass ./cmd/lgrass
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/darwin-amd64/lgrassd ./cmd/lgrassd
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/darwin-arm64/lgrass ./cmd/lgrass
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/darwin-arm64/lgrassd ./cmd/lgrassd

# Host-arch build for local dev iteration, lgrass installed straight to
# ~/.local/bin and lgrassd to ~/.lemongrass/bin. dist cross-compiles arches for packaging;
# this builds for the host's own GOOS/GOARCH only. Used by the root
# Makefile's dev target.
install:
	mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/lgrass ./cmd/lgrass
	mkdir -p $(D_BIN_DIR)
	go build $(LDFLAGS) -o $(D_BIN_DIR)/lgrassd.new ./cmd/lgrassd
	mv -f $(D_BIN_DIR)/lgrassd.new $(D_BIN_DIR)/lgrassd

# Compiles everything and discards the output, since a plain `go build` of a single main package writes its binary into the source directory.
verify:
	go build -a -o /dev/null ./...

clean:
	rm -rf $(DIST)
	rm -f lgrass lgrass.exe lgrassd lgrassd.exe
