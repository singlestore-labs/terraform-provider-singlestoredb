NAME=singlestore
BINARY=terraform-provider-${NAME}

default: install

deps:
	go mod tidy

build:
	go build -o ${BINARY}

install: deps build
	go install .

unit: nocache
	go test -v -short ./...

integration: nocache
	go test -v -run Integration ./...

nocache:
	go clean -testcache