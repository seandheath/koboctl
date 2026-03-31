BINARY := bin/koboctl
CMD := ./cmd/koboctl
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X github.com/seandheath/koboctl/internal/build.Version=$(VERSION)"

.PHONY: build test lint fmt vet clean

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY) $(CMD)

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	goimports -w .

vet:
	go vet ./...

clean:
	rm -rf bin/
