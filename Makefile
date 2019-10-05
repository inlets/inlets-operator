.PHONY: build build-armhf push test verify-codegen
TAG?=latest
REGISTRY?=alexellis

build:
	docker build -t $(REGISTRY)/inlets-operator:$(TAG) . -f Dockerfile

push:
	docker push $(REGISTRY)/inlets-operator:$(TAG)

test:
	go test ./...

verify-codegen:
	./hack/verify-codegen.sh
