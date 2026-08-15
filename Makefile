BINARY  := agis
GO      ?= go

.PHONY: build test vet lint fmt tidy clean

## build: compile the agis binary
build:
	$(GO) build -o bin/$(BINARY) ./cmd/agis

## test: run all tests
test:
	$(GO) test ./...

## vet: run go vet
vet:
	$(GO) vet ./...

## lint: run golangci-lint
lint:
	golangci-lint run

## fmt: format all Go files
fmt:
	$(GO) fmt ./...

## tidy: tidy module dependencies
tidy:
	$(GO) mod tidy

## clean: remove build artifacts
clean:
	rm -rf bin
