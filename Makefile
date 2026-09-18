.PHONY: build run test test-backend test-ui typecheck fmt vet lint \
	package package-host verify-package verify-package-host clean

# VERSION is read from manifest.yaml so the packaged asset name cannot drift
# from the version the release workflow gates the tag against.
BIN := bin/kandev-plugin-forgejo
VERSION := $(shell sed -n 's/^version: *"\(.*\)"/\1/p' manifest.yaml | head -1)
STAGE := .build/stage
PKG_OUT := kandev-plugin-forgejo-$(VERSION).tar.gz

# The sibling kandev checkout the `replace` in go.mod points at. plugin-pack is
# run from INSIDE that directory so its dependencies resolve against kandev's
# own go.sum rather than this module's.
KANDEV_SDK := ../kandev/apps/backend

# Release binaries are stripped and path-trimmed. `-s -w` drops the symbol
# table and DWARF, which is about a third of the binary; Go keeps its pclntab
# either way, so panic traces still carry function names and line numbers.
# `-trimpath` replaces the builder's absolute paths with module-relative ones,
# which keeps builds reproducible and keeps the build machine's directory
# layout out of the shipped artifact.
RELEASE_FLAGS := -trimpath -ldflags="-s -w"

## Build the plugin binary for the host platform (development only). Kandev
## always installs from `make package`/`package-host` output.
build:
	mkdir -p bin
	go build -o $(BIN) ./server/...

run: build
	./$(BIN)

test: test-backend test-ui

test-backend:
	go test ./...

## Builds the bundle first: the manifest test asserts ui/bundle.js exists.
test-ui:
	npm run build:ui
	npx vitest run

typecheck:
	npm run typecheck

fmt:
	gofmt -l .

vet:
	go vet ./...

lint: fmt vet typecheck

## Cross-compile every platform in manifest.yaml's runtime.executables, stage
## manifest.yaml + ui/ + assets/ alongside them, and pack the tree.
package: ui/bundle.js
	rm -rf $(STAGE)
	mkdir -p $(STAGE)/server
	cp manifest.yaml $(STAGE)/manifest.yaml
	cp -r ui $(STAGE)/ui
	cp -r assets $(STAGE)/assets
	rm -rf $(STAGE)/ui/src $(STAGE)/ui/test $(STAGE)/ui/tsconfig.json
	GOOS=linux   GOARCH=amd64 go build $(RELEASE_FLAGS) -o $(STAGE)/server/plugin-linux-amd64       ./server
	GOOS=linux   GOARCH=arm64 go build $(RELEASE_FLAGS) -o $(STAGE)/server/plugin-linux-arm64       ./server
	GOOS=darwin  GOARCH=amd64 go build $(RELEASE_FLAGS) -o $(STAGE)/server/plugin-darwin-amd64      ./server
	GOOS=darwin  GOARCH=arm64 go build $(RELEASE_FLAGS) -o $(STAGE)/server/plugin-darwin-arm64      ./server
	GOOS=windows GOARCH=amd64 go build $(RELEASE_FLAGS) -o $(STAGE)/server/plugin-windows-amd64.exe ./server
	cd $(KANDEV_SDK) && go run ./cmd/plugin-pack -dir $(CURDIR)/$(STAGE) -out $(CURDIR)/$(PKG_OUT)
	rm -rf $(STAGE)
	@echo "Wrote $(PKG_OUT)"

## Host-platform-only package — faster local iteration.
package-host: ui/bundle.js
	rm -rf $(STAGE)
	mkdir -p $(STAGE)/server
	cp manifest.yaml $(STAGE)/manifest.yaml
	cp -r ui $(STAGE)/ui
	cp -r assets $(STAGE)/assets
	rm -rf $(STAGE)/ui/src $(STAGE)/ui/test $(STAGE)/ui/tsconfig.json
	go build $(RELEASE_FLAGS) -o $(STAGE)/server/plugin-$$(go env GOOS)-$$(go env GOARCH)$$(go env GOEXE) ./server
	cd $(KANDEV_SDK) && go run ./cmd/plugin-pack -dir $(CURDIR)/$(STAGE) -out $(CURDIR)/$(PKG_OUT) -platform-only
	rm -rf $(STAGE)
	@echo "Wrote $(PKG_OUT)"

ui/bundle.js: $(wildcard ui/src/*.ts) package.json ui/tsconfig.json
	npm run build:ui

## Build and validate the all-platform archive.
verify-package: package
	@tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
		tar -xzf "$(PKG_OUT)" -C "$$tmp"; \
		test -f "$$tmp/manifest.yaml"; \
		test -f "$$tmp/ui/bundle.js"; \
		test -f "$$tmp/ui/plugin.css"; \
		test -f "$$tmp/assets/icon.svg"; \
		test -f "$$tmp/checksums.txt"; \
		for executable in \
			plugin-linux-amd64 plugin-linux-arm64 \
			plugin-darwin-amd64 plugin-darwin-arm64 \
			plugin-windows-amd64.exe; do \
			test -f "$$tmp/server/$$executable"; \
		done; \
		test ! -e "$$tmp/ui/src"; \
		test ! -e "$$tmp/package.json"; \
		test ! -e "$$tmp/internal"; \
		if command -v sha256sum >/dev/null 2>&1; then \
			(cd "$$tmp" && sha256sum -c checksums.txt); \
		else \
			(cd "$$tmp" && shasum -a 256 -c checksums.txt); \
		fi
	@echo "verify-package OK"

verify-package-host: package-host
	@tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
		tar -xzf "$(PKG_OUT)" -C "$$tmp"; \
		host_executable="plugin-$$(go env GOOS)-$$(go env GOARCH)$$(go env GOEXE)"; \
		test -f "$$tmp/manifest.yaml"; \
		test -f "$$tmp/ui/bundle.js"; \
		test -f "$$tmp/assets/icon.svg"; \
		test -f "$$tmp/checksums.txt"; \
		test -f "$$tmp/server/$$host_executable"; \
		test ! -e "$$tmp/ui/src"; \
		test ! -e "$$tmp/package.json"; \
		if command -v sha256sum >/dev/null 2>&1; then \
			(cd "$$tmp" && sha256sum -c checksums.txt); \
		else \
			(cd "$$tmp" && shasum -a 256 -c checksums.txt); \
		fi
	@echo "verify-package-host OK"

clean:
	rm -rf bin $(STAGE) kandev-plugin-forgejo-*.tar.gz
