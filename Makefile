.PHONY: build push manifest test verify-codegen charts
TAG?=latest
OWNER?=openfaas
SERVER?=ghcr.io
IMG_NAME?=inlets-operator
VERBOSE?=false

LDFLAGS := "-s -w -X github.com/inlets/inlets-operator/pkg/version.Release=$(Version) -X github.com/inlets/inlets-operator/pkg/version.SHA=$(GitCommit)"
PLATFORM?= "linux/amd64,linux/arm/v7,linux/arm64"

Version := $(shell git describe --tags --dirty)
GitCommit := $(shell git rev-parse HEAD)

# docker manifest command will work with Docker CLI 18.03 or newer
# but for now it's still experimental feature so we need to enable that
export DOCKER_CLI_EXPERIMENTAL=enabled

TOOLS_DIR := .tools

GOPATH := $(shell go env GOPATH)
CODEGEN_VERSION := $(shell hack/print-codegen-version.sh)
CODEGEN_PKG := $(GOPATH)/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}


.PHONY: all
all: build

$(TOOLS_DIR)/code-generator.mod: go.mod
	@echo "syncing code-generator tooling version"
	@cd $(TOOLS_DIR) && go mod edit -require "k8s.io/code-generator@${CODEGEN_VERSION}"

${CODEGEN_PKG}: $(TOOLS_DIR)/code-generator.mod
	@echo "(re)installing k8s.io/code-generator-${CODEGEN_VERSION}"
	@cd $(TOOLS_DIR) && go mod download -modfile=code-generator.mod

.PHONY: build-local
build-local:
	@docker buildx create --use --name=multiarch --node multiarch && \
	docker buildx build \
		--progress=plain \
		--build-arg Version=$(Version) --build-arg GitCommit=$(GitCommit) \
		--platform linux/amd64 \
		--output "type=docker,push=false" \
		--tag $(SERVER)/$(OWNER)/$(IMG_NAME):$(Version) .

.PHONY: build
build:
	@echo $(SERVER)/$(OWNER)/$(IMG_NAME):$(Version) && \
	docker buildx create --use --name=multiarch --node multiarch && \
	docker buildx build \
		--progress=plain \
		--build-arg Version=$(Version) --build-arg GitCommit=$(GitCommit) \
		--platform $(PLATFORM) \
		--output "type=image,push=false" \
		--tag $(SERVER)/$(OWNER)/$(IMG_NAME):$(Version) .

.PHONY: push
push:
	@echo $(SERVER)/$(OWNER)/$(IMG_NAME):$(Version) && \
	docker buildx create --use --name=multiarch --node multiarch && \
	docker buildx build \
		--progress=plain \
		--build-arg Version=$(Version) --build-arg GitCommit=$(GitCommit) \
		--platform $(PLATFORM) \
		--output "type=image,push=true" \
		--tag $(SERVER)/$(OWNER)/$(IMG_NAME):$(Version) .

test:
	go test ./...

.PHONY: verify-codegen
verify-codegen: ${CODEGEN_PKG}
	./hack/verify-codegen.sh

.PHONY: update-codegen
update-codegen: ${CODEGEN_PKG}
	./hack/update-codegen.sh

charts:
	cd chart && helm package inlets-operator/
	mv chart/*.tgz docs/
	helm repo index docs --url https://inlets.github.io/inlets-operator/ --merge ./docs/index.yaml

.PHONY: verify-charts
verify-charts:
	@echo Verifying helm charts images in remote registries && \
	arkade chart verify --verbose=$(VERBOSE) -f ./chart/inlets-operator/values.yaml
