FROM golang:1.11

RUN mkdir -p /go/src/github.com/inlets/inlets-operator/

WORKDIR /go/src/github.com/inlets/inlets-operator

COPY . .

ARG OPTS

RUN gofmt -l -d $(find . -type f -name '*.go' -not -path "./vendor/*") && \
  go test -v ./ && \
  VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///') && \
  GIT_COMMIT=$(git rev-list -1 HEAD) && \
  env ${OPTS} CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w \
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

COPY --from=0 /etc/passwd /etc/group /etc/
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 --chown=app:app /scratch-tmp /tmp/
COPY --from=0 /go/src/github.com/inlets/inlets-operator/inlets-operator .

USER app

ENTRYPOINT ["./inlets-operator"]
CMD ["-logtostderr"]
