FROM golang

ENV DIR /go/src/github.com/erikh/border

COPY . ${DIR}
WORKDIR ${DIR}
CMD cd ${DIR} && go test -v ./integration-tests/...