.PHONY: dev build clean

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

dev:
	wails dev -tags webkit2_41

build: frontend
	wails build -tags webkit2_41 -ldflags "$(LDFLAGS)"

frontend:
	cd frontend && npm install && npm run build

clean:
	rm -rf build/
