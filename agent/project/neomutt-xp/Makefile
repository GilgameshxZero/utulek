# One patched source tree, two platform builds. No installation is performed.
PYTHON ?= python3
TOOLCHAIN ?= $(CURDIR)/.obj/deps/llvm_mingw-20260908

.PHONY: all linux windows prepare-linux prepare-windows toolchain verify-sources clean
all: linux windows

prepare-linux:
	$(PYTHON) scripts/prepare.py neomutt lmdb libgpg-error libassuan gpgme sqlite

prepare-windows:
	$(PYTHON) scripts/prepare.py neomutt ncurses libiconv pcre2 openssl lmdb libgpg-error libassuan gpgme sqlite

toolchain:
	$(PYTHON) scripts/prepare.py llvm_mingw

linux: prepare-linux
	$(PYTHON) scripts/build_lmdb.py linux
	$(PYTHON) scripts/build_crypto.py linux
	$(PYTHON) scripts/build.py linux

windows: prepare-windows
	$(PYTHON) scripts/build_deps.py ncurses --toolchain "$(TOOLCHAIN)"
	$(PYTHON) scripts/build_deps.py iconv --toolchain "$(TOOLCHAIN)"
	$(PYTHON) scripts/build_deps.py pcre2 --toolchain "$(TOOLCHAIN)"
	$(PYTHON) scripts/build_openssl.py --toolchain "$(TOOLCHAIN)"
	$(PYTHON) scripts/build_lmdb.py windows --toolchain "$(TOOLCHAIN)"
	$(PYTHON) scripts/build_crypto.py windows --toolchain "$(TOOLCHAIN)"
	$(PYTHON) scripts/build.py windows --toolchain "$(TOOLCHAIN)"

verify-sources:
	$(PYTHON) scripts/verify_sources.py

# Checkpoint snapshots are build artifacts under .obj/checkpoints/.
clean:
	$(RM) -r .obj .bin
