FROM teamserverless/license-check:0.3.6 as license-check

FROM golang:1.13 as builder
ENV CGO_ENABLED=0
ENV GO111MODULE=on

WORKDIR /go/src/github.com/inlets/inlets-operator

# Cache the download before continuing
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY --from=license-check /license-check /usr/bin/

COPY . .

ARG OPTS

RUN gofmt -l -d $(find . -type f -name '*.go' -not -path "./vendor/*")
RUN go test -v ./...
RUN license-check -path ./ --verbose=false "Alex Ellis" "inlets Authors" "inlets Author(s)"
RUN VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///') && \
  GIT_COMMIT=$(git rev-list -1 HEAD) && \
  env ${OPTS} CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w \
  -X github.com/inlets/inlets-operator/pkg/version.Release=${VERSION} \
  -X github.com/inlets/inlets-operator/pkg/version.SHA=${GIT_COMMIT}" \
  -a -installsuffix cgo -o inlets-operator .

# Use distroless as minimal base image to package the binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /go/src/github.com/inlets/inlets-operator/inlets-operator /
USER nonroot:nonroot

CMD ["/inlets-operator"]
