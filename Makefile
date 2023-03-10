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

test: build nocache # Tests depend on the binary & run both unit and integration tests.
	go test -v ./... -covermode=count -coverprofile=coverage.out
	go tool cover -func=coverage.out -o=coverage.out

nocache:
	go clean -testcache

tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.2

lint: tools
	golangci-lint run ./...