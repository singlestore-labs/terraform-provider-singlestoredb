HOSTNAME=registry.terraform.io
NAMESPACE=singlestoredb
NAME=singlestore
BINARY=terraform-provider-${NAME}
VERSION=0.0.0
OS_ARCH=linux_amd64

build:
	go build -o ${BINARY}

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}