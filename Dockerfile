FROM teamserverless/license-check:0.3.6 as license-check

FROM golang:1.13 as builder
ENV CGO_ENABLED=0
ENV GO111MODULE=on
#ENV GOFLAGS=-mod=vendor

COPY --from=license-check /license-check /usr/bin/

RUN mkdir -p /go/src/github.com/inlets/inlets-operator
WORKDIR /go/src/github.com/inlets/inlets-operator

COPY . .

ARG OPTS

RUN gofmt -l -d $(find . -type f -name '*.go' -not -path "./vendor/*")
RUN go test -mod=vendor -v ./...
RUN VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///') && \
  GIT_COMMIT=$(git rev-list -1 HEAD) && \
  env ${OPTS} CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags "-s -w \
  -X github.com/inlets/inlets-operator/pkg/version.Release=${VERSION} \
  -X github.com/inlets/inlets-operator/pkg/version.SHA=${GIT_COMMIT}" \
  -a -installsuffix cgo -o inlets-operator . && \
  addgroup --system app && \
  adduser --system --ingroup app app && \
  mkdir /scratch-tmp

# we can't add user in next stage because it's from scratch
# ca-certificates and tmp folder are also missing in scratch
# so we add all of it here and copy files in next stage

FROM scratch

COPY --from=builder /etc/passwd /etc/group /etc/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder --chown=app:app /scratch-tmp /tmp/
COPY --from=builder /go/src/github.com/inlets/inlets-operator/inlets-operator .

USER app

CMD ["./inlets-operator"]
