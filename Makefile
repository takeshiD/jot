# Reproducible build: pinned deps (go.mod/go.sum), -trimpath, no cgo, no VCS stamp.
GO      ?= go
VERSION ?= 0.1.0
FLAGS    = -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build build-linux-amd64 test clean

build:
	CGO_ENABLED=0 $(GO) build $(FLAGS) -o bin/jot ./cmd/jot

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(FLAGS) -o dist/jot-linux-amd64 ./cmd/jot

test:
	CGO_ENABLED=0 $(GO) test ./...

clean:
	rm -rf bin dist
