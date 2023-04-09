all: test

test-full:
	go test -v ./... -count 1
	make test-integration

test:
	go test -v ./...
	make test-integration

test-integration:
	docker build -t border .
	docker run --rm -e IN_DOCKER=1 -v ${PWD}:/go/src/github.com/erikh/border -v ${PWD}/.docker-pkg:/go/pkg:rw --privileged -it border
