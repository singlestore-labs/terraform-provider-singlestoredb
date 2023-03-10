NAME=singlestore
BINARY=terraform-provider-${NAME}

default: install

deps:
	go mod tidy

build:
	go build -o ${BINARY}

install: deps build
	go install .

unit: build nocache # Unit tests depend on the binary.
	go test -v -short ./...

integration: build nocache # Integration tests depend on the binary.
	go test -v -run Integration ./...

nocache:
	go clean -testcache

tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.2

lint: tools
	golangci-lint run ./...