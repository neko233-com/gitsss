.PHONY: help fmt lint-go lint-js tidy test test-go test-js build build-web package

help:
	@echo "gitsss development targets"
	@echo "  fmt        Format Go and TypeScript sources"
	@echo "  lint-go    Run Go vet"
	@echo "  lint-js    Run TypeScript lint and type checks"
	@echo "  tidy       Tidy the server Go module"
	@echo "  test       Run Go and web unit tests"
	@echo "  build      Build the React client and Go server"
	@echo "  package    Build six local release packages"

fmt:
	cd server && go fmt ./...
	cd web && pnpm run format

lint-go:
	cd server && go vet ./...

lint-js:
	cd web && pnpm run lint
	cd web && pnpm run typecheck

tidy:
	cd server && go mod tidy

test: test-go test-js

test-go:
	cd server && go test ./...

test-js:
	cd web && pnpm run test

build: build-web
	cd server && go build ./cmd/gitsss ./cmd/gitsss-agent ./cmd/gitsss-launcher

build-web:
	cd web && pnpm run build

package:
	powershell -NoProfile -ExecutionPolicy Bypass -File tools/package-gitsss-release.ps1
