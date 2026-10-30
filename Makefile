SHELL := /bin/bash
GO ?= go
BIN ?= cf

.PHONY: test vet build check docs-check docs-build clean

test:
	@$(GO) test ./...

vet:
	@$(GO) vet ./...

build:
	@mkdir -p dist
	@$(GO) build -trimpath -o dist/$(BIN) ./cmd/cf

check: test vet docs-check

docs-check:
	@cd docs && bun run check

docs-build:
	@cd docs && bun run build

clean:
	@rm -rf dist docs/dist docs/.astro .vercel/output
