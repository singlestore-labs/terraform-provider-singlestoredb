NAME=singlestore
BINARY=terraform-provider-${NAME}

UNIT_COVERAGE=unitcoverage.txt
INTEGRATION_COVERAGE=integrationcoverage.txt
COVERAGE=coverage.txt

default: install

deps:
	go mod tidy

build:
	go build -o ${BINARY}

install: deps build
	go install .

unit: build nocache # Unit tests depend on the binary.
	go test -v -short -coverprofile=${UNIT_COVERAGE} ./...

integration: build nocache # Integration tests depend on the binary.
	go test -v -run Integration -coverprofile=${INTEGRATION_COVERAGE} ./...

nocache:
	go clean -testcache

tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.2
	go install github.com/wadey/gocovmerge@b5bfa59ec0adc420475f97f89b58045c721d761c

lint: tools
	golangci-lint run ./...

gocovmerge: tools
	gocovmerge ${UNIT_COVERAGE} ${INTEGRATION_COVERAGE} > ${COVERAGE}