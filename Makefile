.PHONY: build build-ui build-images dev grammars tooling lemongrass

VERSION ?= dev

build-ui:
	cd ui && npm install && npm run build

build: build-ui
	mkdir -p bin
	go build \
	  -ldflags "-X github.com/faizalv/lemongrass/cmd/lemongrass/version.Version=$(VERSION)" \
	  -o bin/lemongrass ./cmd/lemongrass/
	go build -o bin/lemongrass-server ./cmd/http/
	go build -ldflags "-X main.isHost=true" -o bin/lg-hook-host ./cmd/lg-hook/

build-images:
	docker build -f Dockerfile.server -t lemongrass-server:local .
	docker build -f Dockerfile.runner -t lemongrass-runner:local .
	docker build -f Dockerfile.embed  -t lemongrass-embed:local  .
	docker build -f Dockerfile.lang   -t lemongrass-lang:local   .

grammars:
	cd grammars && make all

dev:
	go run ./cmd/http

lemongrass: build build-images
	@if [ -d "$$HOME/.local/bin" ]; then \
		install -m 755 bin/lemongrass $$HOME/.local/bin/lemongrass; \
		install -m 755 bin/lg-hook-host $$HOME/.local/bin/lg-hook-host; \
	else \
		install -m 755 bin/lemongrass /usr/local/bin/lemongrass; \
		install -m 755 bin/lg-hook-host /usr/local/bin/lg-hook-host; \
	fi
	bin/lemongrass _scaffold
	@echo ""
	@echo "Run: lemongrass up"
