VERSION := $(shell node -p "require('./ui/package.json').version")

.PHONY: build-linux clean

build-linux:
	$(MAKE) -C lgrass dist VERSION=$(VERSION)
	npm --prefix ui run build:linux

clean:
	$(MAKE) -C lgrass clean
	rm -rf ui/out ui/dist
