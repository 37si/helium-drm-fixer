GO      ?= go
BINARY  := hdf
GOOS    := $(shell $(GO) env GOOS)
GOARCH  := $(shell $(GO) env GOARCH)
EXE     := $(if $(filter windows,$(GOOS)),.exe,)

REPO    ?= 37si/helium-drm-fixer
VERSION ?= dev

LDFLAGS := -s -w -X main.version=$(VERSION) -X main.repo=$(REPO)
MK      := $(GO) run ./tools/mk

export GO
export GOOS
export GOARCH

.DEFAULT_GOAL := help
.PHONY: help build run test vet fmt lint check install release clean

help:
	@$(MK) help -version $(VERSION)

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY)$(EXE) .
	@echo built bin/$(BINARY)$(EXE)

run:
	$(GO) run .

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint:
	@$(MK) lint

check: lint test

install:
	$(GO) install -ldflags "$(LDFLAGS)" .

release:
	@$(MK) release -version $(VERSION) -repo $(REPO)

clean:
	@$(MK) clean
