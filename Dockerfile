FROM golang

ARG DIR=/go/src/github.com/erikh/border

WORKDIR ${DIR}
RUN curl https://get.docker.com | bash
CMD sh integration-tests/run.sh
