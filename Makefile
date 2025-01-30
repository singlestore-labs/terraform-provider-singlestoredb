NAME=singlestoredb
BINARY=terraform-provider-${NAME}
COVERAGE=coverage.out

default: install

deps:
	go mod tidy

build:
	go build -o ${BINARY}

install: deps build
	go install .

unit: install nocache # Unit tests depend on the binary.
	go test -race -v -timeout=5m -short ./... -coverprofile=${COVERAGE} -covermode=atomic

integration: install nocache # Integration tests depend on the binary.
	go test -race -v -timeout=2h -run Integration ./... -coverprofile=${COVERAGE} -covermode=atomic

nocache:
	go clean -testcache

generate: tools
	terraform fmt -recursive ./examples/
	tfplugindocs

tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.63.4
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.14.1

lint-fast: tools
	golangci-lint run --fast ./...

lint: tools
	golangci-lint run --fast ./...

gencheck:
	git diff --compact-summary --exit-code || \
            (echo; echo "Unexpected difference in directories after code generation. Run 'make generate' command and commit."; exit 1)
	tfplugindocs validate
	./.validate_readme