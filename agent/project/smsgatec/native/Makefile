.DEFAULT_GOAL := all
.PHONY: all windows test test-windows clean

PROJECT_DIR := $(abspath ..)
export GOCACHE := $(PROJECT_DIR)/.obj/go_cache
export GOMODCACHE := $(PROJECT_DIR)/.obj/go_mod
export GOTMPDIR := $(PROJECT_DIR)/.obj/tmp

.PHONY: prepare
prepare:
	mkdir -p ../.bin "$(GOTMPDIR)"

# Go optimizations are enabled by default. Strip debug symbols and build paths.
all: prepare
	CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o ../.bin/smsgatec .

windows: prepare
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o ../.bin/smsgatec.exe .

test: prepare
	go test ./...

test-windows: prepare
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -c -o ../.bin/smsgatec-tests.exe

clean:
	chmod -R u+w ../.obj 2>/dev/null || true
	rm -rf ../.obj ../.bin
