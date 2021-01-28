#
# Commands
#

export KO ?= ko
export KUBECTL ?= kubectl
export GIT ?= git
export GO ?= go
export MKDIR_P ?= mkdir -p
export RM_F ?= rm -f
export SHELLCHECK ?= shellcheck

#
# Variables
#

export PVPOOL_VERSION ?= $(shell $(GIT) describe --tags --always --dirty)

export KO_DOCKER_REPO ?= ko.local
export GOFLAGS ?=

#
#
#

ARTIFACTS_DIR := artifacts

#
# Targets
#

.PHONY: all
all: build

$(ARTIFACTS_DIR):
	$(MKDIR_P) $@

.PHONY: generate
generate: $(ARTIFACTS_DIR)
	$(GO) generate ./...

.PHONY: build-manifest-%
build-manifest-%: export PVPOOL_VERSION := $(PVPOOL_VERSION)
build-manifest-%: generate
	$(GO) run sigs.k8s.io/kustomize/kustomize/v3 build manifests/$* \
		| $(KO) resolve -f - >$(ARTIFACTS_DIR)/$*.yaml

.PHONY: build
build: build-manifest-release build-manifest-debug

.PHONY: apply
apply: build
	$(KUBECTL) apply -f $(ARTIFACTS_DIR)/release.yaml

.PHONY: check
check: generate
	scripts/check

.PHONY: test
test: generate
	scripts/test

.PHONY: clean
clean:
	$(RM_F) -r $(ARTIFACTS_DIR)/
