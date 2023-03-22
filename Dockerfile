FROM golang

ENV DIR /go/src/github.com/erikh/border
ENV IN_DOCKER 1

COPY . ${DIR}
WORKDIR ${DIR}
CMD cd ${DIR} && go test -v ./integration-tests/...
