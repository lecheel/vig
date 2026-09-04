.PHONY: all run test build build-run build-windows package-windows setup-runtime setup-runtime-windows

# Git version metadata matching ops.sh
COMMIT_HASH ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "")
DIRTY       ?= $(shell git status --porcelain -uno 2>/dev/null | grep -q . && echo "true" || echo "false")
LDFLAGS     ?= -s -w \
               -X github.com/firstrow/wig/version.Version=$(GIT_VERSION) \
               -X github.com/firstrow/wig/version.CommitHash=$(COMMIT_HASH) \
               -X github.com/firstrow/wig/version.Dirty=$(DIRTY)

# Cross-compiler detection (MinGW on Linux/macOS, native gcc on Windows)
ifeq ($(OS),Windows_NT)
	WIN_CC  ?= gcc
	WIN_CXX ?= g++
else
	WIN_CC  ?= x86_64-w64-mingw32-gcc
	WIN_CXX ?= x86_64-w64-mingw32-g++
endif

run:
	go run cmd/main.go > /tmp/wig.panic.txt 2>&1

test:
	go test -v ./... -count=1

build:
	GOEXPERIMENT=greenteagc go build -ldflags "$(LDFLAGS)" -o wig ./cmd
	mkdir -p ~/go/bin
	mv ./wig ~/go/bin/wig

build-run: build
	wig > /tmp/wig.panic.txt 2>&1

build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(WIN_CC) CXX=$(WIN_CXX) go build -ldflags "$(LDFLAGS)" -o wig.exe ./cmd

package-windows: build-windows
	zip -9 wig-windows-amd64.zip wig.exe

setup-runtime:
	mkdir -p ~/.config/wig
	cp -r ./runtime/* ~/.config/wig/

setup-runtime-windows:
	mkdir -p "$$APPDATA/wig"
	cp -r ./runtime/* "$$APPDATA/wig/"
