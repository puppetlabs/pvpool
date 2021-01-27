#
# Commands
#

export GO ?= go
export SHELLCHECK ?= shellcheck

#
# Variables
#

export GOFLAGS ?=


#
# Targets
#

.PHONY: all
all: build

.PHONY: generate
generate:
	$(GO) generate ./...

.PHONY: build
build: generate

.PHONY: check
check: generate
	scripts/check

.PHONY: test
test: generate
	scripts/test

.PHONY: clean
clean:
