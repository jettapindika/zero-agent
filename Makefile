.PHONY: dev build clean test run

run: build
	./bin/zero

dev: build
	./bin/zero

build:
	go build -o bin/zero ./apps/cli
	go build -o bin/zero-server ./services/core/cmd/server

clean:
	rm -rf bin/ apps/web/.next apps/web/node_modules/.cache

test:
	go test ./packages/sdk-go/...
	go test ./services/core/...
	go test ./apps/cli/...
