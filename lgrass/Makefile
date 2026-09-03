VERSION ?= dev
DIST := dist

.PHONY: dist clean

dist:
	mkdir -p $(DIST)/linux-amd64 $(DIST)/linux-arm64
	GOOS=linux GOARCH=amd64 go build \
	  -ldflags "-X github.com/faizalv/lemongrass/cmd/lgrass/version.Version=$(VERSION)" \
	  -o $(DIST)/linux-amd64/lgrass ./cmd/lgrass
	GOOS=linux GOARCH=arm64 go build \
	  -ldflags "-X github.com/faizalv/lemongrass/cmd/lgrass/version.Version=$(VERSION)" \
	  -o $(DIST)/linux-arm64/lgrass ./cmd/lgrass

clean:
	rm -rf $(DIST)
