.PHONY: dev build clean test run install uninstall desktop-dev desktop-build install-opencode-config

PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

run: build
	./bin/zero

dev: build
	./bin/zero

build:
	go build -o bin/zero ./apps/cli
	go build -o bin/zero-server ./services/core/cmd/server

desktop-dev: build
	pnpm --filter @zero-agent/desktop tauri:dev

desktop-build: build
	pnpm --filter @zero-agent/desktop tauri:build

install: build
	mkdir -p "$(BINDIR)"
	install -m 0755 bin/zero "$(BINDIR)/zero"
	install -m 0755 bin/zero-server "$(BINDIR)/zero-server"
	@echo "Installed zero to $(BINDIR)/zero"
	@echo "If zero is not found, add $(BINDIR) to PATH."

uninstall:
	rm -f "$(BINDIR)/zero" "$(BINDIR)/zero-server"

clean:
	rm -rf bin/ apps/web/.next apps/web/node_modules/.cache

test:
	go test ./packages/sdk-go/...
	go test ./services/core/...
	go test ./apps/cli/...

# Install ~/.config/opencode/* into ~/.config/zero/* (non-destructive).
install-opencode-config: build
	./bin/zero install-opencode
