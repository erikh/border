all: test

test:
	go test -v ./pkg/...
	make test-integration

test-integration:
	docker build -t border .
	docker run -it border
