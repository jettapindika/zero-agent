.PHONY: dev build clean test run install uninstall desktop-dev desktop-build desktop-build-windows desktop-icons install-app install-all install-opencode-config library library-test

PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
APPDIR ?= $(HOME)/Applications

# Build-time secrets are sourced from .build-secrets (gitignored). When the
# file is missing (fresh OSS clone, contributor build) ldflags is empty and
# the daemon falls back to env vars / BYO Google OAuth client.
SERVER_PKG := github.com/zero-agent/core/pkg/server
ifneq (,$(wildcard .build-secrets))
GOOGLE_CLIENT_ID     := $(shell . ./.build-secrets && printf %s $$GOOGLE_CLIENT_ID)
GOOGLE_CLIENT_SECRET := $(shell . ./.build-secrets && printf %s $$GOOGLE_CLIENT_SECRET)
LDFLAGS              := -X '$(SERVER_PKG).defaultGoogleClientID=$(GOOGLE_CLIENT_ID)' -X '$(SERVER_PKG).defaultGoogleClientSecret=$(GOOGLE_CLIENT_SECRET)'
else
LDFLAGS              :=
endif

run: build
	./bin/zero

dev: build
	./bin/zero

build:
	go build -ldflags "$(LDFLAGS)" -o bin/zero ./apps/cli
	go build -ldflags "$(LDFLAGS)" -o bin/zero-server ./services/core/cmd/server

library:
	go build -o bin/library ./tools/library/cmd/library

library-test:
	go test ./tools/library/...

desktop-dev: build
	pnpm --filter @zero-agent/desktop tauri:dev

desktop-build: build
	pnpm --filter @zero-agent/desktop tauri:build

# Cross-compile Windows installer (.exe via NSIS) from macOS or Linux.
# Requires: nsis, llvm, rustup target add x86_64-pc-windows-msvc, cargo-xwin.
# Output: apps/desktop/src-tauri/target/x86_64-pc-windows-msvc/release/bundle/nsis/
desktop-build-windows: build
	@command -v makensis >/dev/null 2>&1 || { echo "ERROR: makensis not found. Install with: brew install nsis (macOS) or apt install nsis (Linux)"; exit 1; }
	@command -v cargo-xwin >/dev/null 2>&1 || { echo "ERROR: cargo-xwin not found. Install with: cargo install --locked cargo-xwin"; exit 1; }
	@rustup target list --installed | grep -q x86_64-pc-windows-msvc || { echo "ERROR: missing rust target. Install with: rustup target add x86_64-pc-windows-msvc"; exit 1; }
	pnpm --filter @zero-agent/desktop exec tauri build --runner cargo-xwin --target x86_64-pc-windows-msvc

# Regenerate per-platform icons from src-tauri/icons/source.png (or source.svg).
# Required after editing the source mark; produces icon.ico, icon.icns,
# Android/iOS variants, and Windows Store tile assets.
desktop-icons:
	@if [ ! -f apps/desktop/src-tauri/icons/source.png ]; then \
		command -v magick >/dev/null 2>&1 || { echo "ERROR: ImageMagick not found. Install with: brew install imagemagick"; exit 1; }; \
		magick -background none -density 300 apps/desktop/src-tauri/icons/source.svg \
			-resize 1024x1024 -define png:color-type=6 -depth 8 \
			PNG32:apps/desktop/src-tauri/icons/source.png; \
	fi
	pnpm --filter @zero-agent/desktop exec tauri icon src-tauri/icons/source.png

install: build
	mkdir -p "$(BINDIR)"
	install -m 0755 bin/zero "$(BINDIR)/zero"
	install -m 0755 bin/zero-server "$(BINDIR)/zero-server"
	@echo "Installed zero to $(BINDIR)/zero"
	@echo "If zero is not found, add $(BINDIR) to PATH."

install-app: desktop-build
	@bundle="apps/desktop/src-tauri/target/release/bundle/macos/Zero.app"; \
	if [ ! -d "$$bundle" ]; then \
		echo "Zero.app not found at $$bundle — desktop-build may have failed."; \
		exit 1; \
	fi; \
	mkdir -p "$(APPDIR)"; \
	rm -rf "$(APPDIR)/Zero.app"; \
	cp -R "$$bundle" "$(APPDIR)/Zero.app"; \
	echo "Installed Zero.app to $(APPDIR)/Zero.app"

install-all: install install-app
	@echo ""
	@echo "Zero is installed. Try:"
	@echo "  zero status     # check daemon health"
	@echo "  zero .          # open desktop in current folder"

uninstall:
	rm -f "$(BINDIR)/zero" "$(BINDIR)/zero-server"
	rm -rf "$(APPDIR)/Zero.app"

clean:
	rm -rf bin/ apps/web/.next apps/web/node_modules/.cache

test:
	go test ./packages/sdk-go/...
	go test ./services/core/...
	go test ./apps/cli/...
	go test ./tools/library/...

# Install ~/.config/opencode/* into ~/.config/zero/* (non-destructive).
install-opencode-config: build
	./bin/zero install-opencode
