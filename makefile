.PHONY: build build-ui dev tooling lemongrass

build-ui:
	cd ui && npm install && npm run build

build: build-ui
	mkdir -p bin
	go build -o bin/server ./cmd/http

dev:
	go run ./cmd/http

tooling:
	mkdir -p bin
	go build -o bin/migrategen ./cmd/tooling/migrategen
	go build -o bin/domigrate ./cmd/tooling/domigrate

lemongrass: build-ui
	docker build -f Dockerfile.server -t lemongrass-server:latest .
	docker build -f Dockerfile.runner -t lemongrass-runner:latest .
	mkdir -p bin
	go build -o bin/lemongrass ./cmd/lemongrass
	@if [ -d "$$HOME/.local/bin" ]; then \
		install -m 755 bin/lemongrass $$HOME/.local/bin/lemongrass; \
	else \
		install -m 755 bin/lemongrass /usr/local/bin/lemongrass; \
	fi
	bin/lemongrass _scaffold
	@echo ""
	@echo "Run: lemongrass up"
