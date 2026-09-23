VERSION := $(shell node -p "require('./ui/package.json').version")

.PHONY: build-linux build-mac dist-linux dist-darwin dev clean

dist-linux:
	$(MAKE) -C lgrass dist-linux VERSION=$(VERSION)
	$(MAKE) -C lgrassconf dist-linux

dist-darwin:
	$(MAKE) -C lgrass dist-darwin VERSION=$(VERSION)
	$(MAKE) -C lgrassconf dist-darwin

build-linux: dist-linux
	npm --prefix ui run build:linux

build-mac: dist-darwin
	npm --prefix ui run build:mac

dev:
	$(MAKE) -C lgrassconf install
	$(MAKE) -C lgrass install
	npm --prefix ui run dev

clean:
	$(MAKE) -C lgrass clean
	$(MAKE) -C lgrassconf clean
	rm -rf ui/out ui/dist
