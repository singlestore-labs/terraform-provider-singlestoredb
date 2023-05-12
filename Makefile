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

generate: tools
	terraform fmt -recursive ./examples/
	tfplugindocs

tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.2
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.14.1

lint: tools
	golangci-lint run ./...

gencheck:
	git diff --compact-summary --exit-code || \
            (echo; echo "Unexpected difference in directories after code generation. Run 'make generate' command and commit."; exit 1)
	tfplugindocs validate