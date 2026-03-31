BINARY := bin/koboctl
CMD := ./cmd/koboctl

.PHONY: build test lint fmt vet clean

build:
	CGO_ENABLED=0 go build -o $(BINARY) $(CMD)

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
