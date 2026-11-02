GO ?= go
VERSION ?= dev
PREFIX ?= $(HOME)/.local
BINARY = bin/syskline$(shell $(GO) env GOEXE)
LDFLAGS = -s -w -X main.version=$(VERSION)

.PHONY: build test race vet check release install clean

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/syskline

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

check: test vet

release:
	VERSION='$(VERSION)' GO='$(GO)' ./scripts/build.sh

install: build
	PREFIX='$(PREFIX)' ./scripts/install.sh $(BINARY)

clean:
	$(GO) clean
