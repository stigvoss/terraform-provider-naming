default: build

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

# Includes tests that run Terraform, so a terraform binary (1.8+) must be on PATH.
test:
	go test -v -cover -timeout=120s -parallel=10 ./...

.PHONY: build install lint generate fmt test
