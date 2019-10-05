FROM golang:1.13 as builder

RUN mkdir -p /go/src/github.com/alexellis/inlets-operator/

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY . .

RUN gofmt -l -d $(find . -type f -name '*.go') && \
  VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///') && \
  GIT_COMMIT=$(git rev-list -1 HEAD) && \
  CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -ldflags "-s -w \
  -X github.com/alexellis/inlets-operator/pkg/version.Release=${VERSION} \
  -X github.com/alexellis/inlets-operator/pkg/version.SHA=${GIT_COMMIT}" \
  -a -installsuffix cgo -o inlets-operator .

FROM alpine:3.10

RUN addgroup -S app \
    && adduser -S -g app app \
    && apk --no-cache add ca-certificates

WORKDIR /home/app

COPY --from=0 /workspace/inlets-operator .

RUN chown -R app:app ./

USER app

ENTRYPOINT ["./inlets-operator"]
CMD ["-logtostderr"]
