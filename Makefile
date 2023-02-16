NAME=singlestore
BINARY=terraform-provider-${NAME}

deps:
	go mod tidy

build:
	go build -o ${BINARY}

install: deps build
	go install .