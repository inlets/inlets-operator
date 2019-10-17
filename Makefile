.PHONY: build push manifest test verify-codegen charts
TAG?=latest

# docker manifest command will work with Docker CLI 18.03 or newer
# but for now it's still experimental feature so we need to enable that
export DOCKER_CLI_EXPERIMENTAL=enabled

build:
	docker build -t inlets/inlets-operator:$(TAG)-amd64 . -f Dockerfile
	docker build --build-arg OPTS="GOARCH=arm64" -t inlets/inlets-operator:$(TAG)-arm64 . -f Dockerfile
	docker build --build-arg OPTS="GOARCH=arm GOARM=6" -t inlets/inlets-operator:$(TAG)-armhf . -f Dockerfile

push:
	docker push inlets/inlets-operator:$(TAG)-amd64
	docker push inlets/inlets-operator:$(TAG)-arm64
	docker push inlets/inlets-operator:$(TAG)-armhf

manifest:
	docker manifest create --amend inlets/inlets-operator:$(TAG) \
		inlets/inlets-operator:$(TAG)-amd64 \
		inlets/inlets-operator:$(TAG)-arm64 \
		inlets/inlets-operator:$(TAG)-armhf
	docker manifest annotate inlets/inlets-operator:$(TAG) inlets/inlets-operator:$(TAG)-arm64 --os linux --arch arm64
	docker manifest annotate inlets/inlets-operator:$(TAG) inlets/inlets-operator:$(TAG)-armhf --os linux --arch arm --variant v6
	docker manifest push -p inlets/inlets-operator:$(TAG)

test:
	go test ./...

verify-codegen:
	./hack/verify-codegen.sh

charts:
	cd chart && helm package inlets-operator/
	mv chart/*.tgz docs/
	helm repo index docs --url https://inlets.github.io/inlets-operator/ --merge ./docs/index.yaml

