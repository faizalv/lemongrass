VERSION ?= dev
DIST := dist
BIN_DIR := $(HOME)/.local/bin

.PHONY: dist install clean

dist:
	mkdir -p $(DIST)/linux-amd64 $(DIST)/linux-arm64
	GOOS=linux GOARCH=amd64 go build -o $(DIST)/linux-amd64/lgconf .
	GOOS=linux GOARCH=arm64 go build -o $(DIST)/linux-arm64/lgconf .

# Builds beside the running binary and renames over it, since a running executable can't be overwritten in place.
install:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/lgconf.new .
	mv -f $(BIN_DIR)/lgconf.new $(BIN_DIR)/lgconf
	$(BIN_DIR)/lgconf install

clean:
	rm -rf $(DIST)
