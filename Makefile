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
export PVPOOL_TEST_E2E_KUBECONFIG ?=
export PVPOOL_TEST_E2E_STORAGE_CLASS_NAME ?=

export KO_DOCKER_REPO ?= ko.local
export GOFLAGS ?=

#
#
#

ARTIFACTS_DIR := artifacts
MANIFEST_DIRS := $(wildcard manifests/*)

BUILD_MANIFEST_TARGETS := $(addprefix build-manifest-,$(notdir $(MANIFEST_DIRS)))
APPLY_MANIFEST_TARGETS := $(addprefix apply-,$(notdir $(MANIFEST_DIRS)))

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

.PHONY: $(BUILD_MANIFEST_TARGETS)
$(BUILD_MANIFEST_TARGETS): build-manifest-%: generate
	$(GO) run sigs.k8s.io/kustomize/kustomize/v3 build manifests/$* \
		| $(KO) resolve -f - >$(ARTIFACTS_DIR)/pvpool-$*-$(PVPOOL_VERSION).yaml

.PHONY: build
build: build-manifest-release build-manifest-debug

.PHONY: $(APPLY_MANIFEST_TARGETS)
$(APPLY_MANIFEST_TARGETS): apply-%: build-manifest-%
	$(KUBECTL) apply -f $(ARTIFACTS_DIR)/pvpool-$*-$(PVPOOL_VERSION).yaml --prune -l app.kubernetes.io/name=pvpool

.PHONY: apply
apply: apply-debug

.PHONY: check
check: generate
	scripts/check

.PHONY: test
ifeq ($(PVPOOL_TEST_E2E_KUBECONFIG),)
test: generate
	scripts/test
else
test: export KUBECONFIG := $(PVPOOL_TEST_E2E_KUBECONFIG)
ifeq ($(PVPOOL_TEST_E2E_STORAGE_CLASS_NAME),)
test: apply-test
	$(KUBECTL) wait --timeout=180s -n local-path-storage --for=condition=available deployments --all
else
test: apply-debug
endif
	$(KUBECTL) wait --timeout=180s -n pvpool --for=condition=available deployments --all
	scripts/test
endif

.PHONY: clean
clean:
	$(RM_F) -r $(ARTIFACTS_DIR)/
