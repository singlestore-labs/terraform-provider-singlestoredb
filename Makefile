NAME=singlestoredb
BINARY=terraform-provider-${NAME}

default: install

deps:
	go mod tidy

build:
	go build -o ${BINARY}

install: deps build
	go install .

unit: install nocache # Unit tests depend on the binary.
	go test -v -timeout=5m -short ./...

integration: install nocache # Integration tests depend on the binary.
	go test -v -timeout=2h -run Integration ./...

nocache:
	go clean -testcache

tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.2

lint: tools
	golangci-lint run ./...