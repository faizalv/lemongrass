VERSION ?= dev
DIST := dist
BIN_DIR := $(HOME)/.local/bin

.PHONY: dist install clean

dist:
	mkdir -p $(DIST)/linux-amd64 $(DIST)/linux-arm64
	GOOS=linux GOARCH=amd64 go build -o $(DIST)/linux-amd64/lgrassconf .
	GOOS=linux GOARCH=arm64 go build -o $(DIST)/linux-arm64/lgrassconf .

# Builds beside the running binary and renames over it, since a running executable can't be overwritten in place.
install:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/lgrassconf.new .
	mv -f $(BIN_DIR)/lgrassconf.new $(BIN_DIR)/lgrassconf
	$(BIN_DIR)/lgrassconf install

clean:
	rm -rf $(DIST)
