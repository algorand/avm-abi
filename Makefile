
build:
	go build -v ./...

test:
	go test ./... -race

test-coverage:
	$(if $(output),,$(error "output" variable not set))
	go test ./... -race -covermode=atomic -coverprofile=$(output)

fmt:
	go fmt ./...

lint:
	golangci-lint run && golangci-lint run -c .golangci-warnings.yml

.PHONY: build test fmt lint
