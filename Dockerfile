FROM golang

ARG DIR=/go/src/github.com/erikh/border

WORKDIR ${DIR}
CMD go test -v ./integration-tests/...
