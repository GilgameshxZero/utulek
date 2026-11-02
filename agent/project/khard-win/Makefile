.DEFAULT_GOAL := all
.PHONY: all windows test test-windows clean

export GOMODCACHE := $(CURDIR)/.obj/mod
export GOCACHE := $(CURDIR)/.obj/cache
export GOTMPDIR := $(CURDIR)/.obj/tmp

.PHONY: prepare
prepare:
	mkdir -p .bin "$(GOTMPDIR)"

all: prepare
	mkdir -p .bin
	go build -trimpath -ldflags='-s -w' -o .bin/khard .

windows: prepare
	mkdir -p .bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o .bin/khard.exe .

test: prepare
	go test ./...

test-windows: prepare
	mkdir -p .bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -c -o .bin/khard-tests.exe .

clean:
	chmod -R u+w .obj 2>/dev/null || true
	rm -rf .obj .bin
