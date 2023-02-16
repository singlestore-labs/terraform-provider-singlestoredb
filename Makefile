NAME=singlestore
BINARY=terraform-provider-${NAME}

build:
	go build -o ${BINARY}

install: build
	go install .