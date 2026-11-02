.PHONY: all clean prepare
export GOCACHE := $(CURDIR)/.obj/go_cache
export GOMODCACHE := $(CURDIR)/.obj/go_mod
export GOTMPDIR := $(CURDIR)/.obj/tmp

all: .bin/smsgate-gv

prepare:
	mkdir -p .bin "$(GOTMPDIR)"

.bin/smsgate-gv: $(wildcard *.go) go.mod go.sum | prepare
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o $@ .

clean:
	chmod -R u+w .obj 2>/dev/null || true
	rm -rf .obj .bin
