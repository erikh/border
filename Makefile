all: test

test:
	go test -v ./...
	make test-integration

test-integration:
	docker build -t border .
	docker run -e IN_DOCKER=1 -v ${PWD}/.docker-pkg:/go/pkg:rw --cap-add NET_ADMIN -it border
