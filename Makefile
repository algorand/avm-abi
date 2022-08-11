
build:
	go build -v ./...

test:
	go test ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run && golangci-lint run -c .golangci-warnings.yml

.PHONY: build test fmt lint
