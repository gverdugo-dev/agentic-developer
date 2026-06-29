# Version is taken from git (latest tag) unless passed explicitly:
#   make release VERSION=v0.1.0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
MODULE  := agentic-developer
LDFLAGS := -s -w -X $(MODULE)/internal/cli.Version=$(VERSION)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

# `make run new skill foo` forwards the bare words after `run` to adev. Those
# words would otherwise be parsed as make targets, so when run is the first goal
# we declare them as no-op rules. The block is scoped to run-first, so it never
# affects any other target. Flags (--scope ...) can't be passed positionally
# because make consumes them itself; pass those through ARGS instead, e.g.
# make run new skill foo ARGS="--scope local".
ifeq (run,$(firstword $(MAKECMDGOALS)))
  RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  $(eval $(RUN_ARGS):;@:)
endif

run: build
	@go run -ldflags "$(LDFLAGS)" ./cmd/adev $(RUN_ARGS) $(ARGS)

build:
	@go build -ldflags "$(LDFLAGS)" -o ./bin/adev ./cmd/adev

install:
	@go install -ldflags "$(LDFLAGS)" ./cmd/adev

# release cross-compiles every target into dist/, archives each, and writes
# checksums. The release workflow runs this same target so the build matrix has
# a single source of truth.
release:
	@rm -rf dist && mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; bin=adev; \
		[ "$$os" = windows ] && bin=adev.exe; \
		echo "building $$os/$$arch ($(VERSION)) ..."; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build -trimpath -ldflags "$(LDFLAGS)" -o "$$bin" ./cmd/adev || exit 1; \
		if [ "$$os" = windows ]; then \
			zip -q "dist/adev_$${os}_$${arch}.zip" "$$bin"; \
		else \
			tar -czf "dist/adev_$${os}_$${arch}.tar.gz" "$$bin"; \
		fi; \
		rm -f "$$bin"; \
	done
	@cd dist && sha256sum * > checksums.txt
	@echo "release artifacts in dist/ (version $(VERSION))"
