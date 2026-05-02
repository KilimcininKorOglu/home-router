BINARY  := home-router
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: build test lint clean dev cross install iso release check

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/home-router

dev:
	go build -o $(BINARY) ./cmd/home-router

test:
	go test ./... -race -count=1

lint:
	golangci-lint run

clean:
	rm -f $(BINARY)

cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/home-router

install: cross
	sudo bash deploy/install.sh ./$(BINARY)

iso: cross
	bash deploy/iso/build-iso.sh $(DEBIAN_ISO) ./$(BINARY)

release: cross
	mkdir -p dist
	tar czf dist/$(BINARY)-$(VERSION)-linux-amd64.tar.gz $(BINARY) deploy/ configs/ web/locales/
	@echo "Release archive: dist/$(BINARY)-$(VERSION)-linux-amd64.tar.gz"

check:
	sudo bash deploy/install.sh --check
