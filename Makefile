.PHONY: build build-armhf push test verify-codegen
TAG?=latest

build:
	docker build -t alexellis/inlets-operator:$(TAG) . -f Dockerfile

push:
	docker push alexellis/inlets-operator:$(TAG)

test:
	go test ./...

verify-codegen:
	./hack/verify-codegen.sh
