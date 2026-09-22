VERSION := $(shell node -p "require('./ui/package.json').version")

.PHONY: build-linux dev clean

build-linux:
	$(MAKE) -C lgrass dist VERSION=$(VERSION)
	$(MAKE) -C lgrassconf dist
	npm --prefix ui run build:linux

dev:
	$(MAKE) -C lgrassconf install
	$(MAKE) -C lgrass install
	npm --prefix ui run dev

clean:
	$(MAKE) -C lgrass clean
	$(MAKE) -C lgrassconf clean
	rm -rf ui/out ui/dist
