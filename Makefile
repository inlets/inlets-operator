.PHONY: build build-armhf push manifest test verify-codegen
TAG?=latest

# docker manifest command will work with Docker CLI 18.03 or newer
# but for now it's still experimental feature so we need to enable that
export DOCKER_CLI_EXPERIMENTAL=enabled

build:
	docker build -t alexellis/inlets-operator:$(TAG)-amd64 . -f Dockerfile
	docker build --build-arg OPTS="GOARCH=arm64" -t alexellis/inlets-operator:$(TAG)-arm64 . -f Dockerfile
	docker build --build-arg OPTS="GOARCH=arm GOARM=6" -t alexellis/inlets-operator:$(TAG)-armhf . -f Dockerfile

push:
	docker push alexellis/inlets-operator:$(TAG)-amd64
	docker push alexellis/inlets-operator:$(TAG)-arm64
	docker push alexellis/inlets-operator:$(TAG)-armhf

manifest:
	docker manifest create --amend alexellis/inlets-operator:$(TAG) \
		alexellis/inlets-operator:$(TAG)-amd64 \
		alexellis/inlets-operator:$(TAG)-arm64 \
		alexellis/inlets-operator:$(TAG)-armhf
	docker manifest annotate alexellis/inlets-operator:$(TAG) alexellis/inlets-operator:$(TAG)-arm64 --os linux --arch arm64
	docker manifest annotate alexellis/inlets-operator:$(TAG) alexellis/inlets-operator:$(TAG)-armhf --os linux --arch arm --variant v6
	docker manifest push alexellis/inlets-operator:$(TAG)

test:
	go test ./...

verify-codegen:
	./hack/verify-codegen.sh
