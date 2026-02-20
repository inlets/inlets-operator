FROM --platform=${BUILDPLATFORM:-linux/amd64} ghcr.io/openfaas/license-check:0.4.1 as license-check
FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25 as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

ARG Version
ARG GitCommit

ENV CGO_ENABLED=0
ENV GO111MODULE=on

COPY --from=license-check /license-check /usr/bin/

RUN mkdir -p /go/src/github.com/inlets/inlets-operator
WORKDIR /go/src/github.com/inlets/inlets-operator

# Cache the download before continuing
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY pkg  pkg
COPY main.go  main.go
COPY image_test.go  image_test.go
COPY controller.go  controller.go
COPY validate.go validate.go
COPY validate_test.go validate_test.go
COPY config.go  config.go
COPY config_test.go  config_test.go
COPY userdata.go userdata.go
COPY userdata_test.go userdata_test.go

RUN gofmt -l -d $(find . -type f -name '*.go' -not -path "./vendor/*")

RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
  go test -v ./...

RUN license-check -path ./ --verbose=false "Alex Ellis" "inlets Authors" "inlets Author(s)"

RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
  go build -ldflags "-s -w -X github.com/inlets/inlets-operator/pkg/version.Release=${Version} -X github.com/inlets/inlets-operator/pkg/version.SHA=${GitCommit}" -o /usr/bin/inlets-operator .

FROM --platform=${BUILDPLATFORM:-linux/amd64} gcr.io/distroless/static:nonroot

LABEL org.opencontainers.image.source=https://github.com/inlets/inlets-operator

WORKDIR /
COPY --from=builder /usr/bin/inlets-operator /
USER nonroot:nonroot

CMD ["/inlets-operator"]
