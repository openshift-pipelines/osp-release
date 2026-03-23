BIN := bin/osp-release

.PHONY: build test lint release-snapshot

build:
	go build -o $(BIN) ./cmd/osp-release

test:
	go test ./...

lint:
	golangci-lint run

release-snapshot:
	goreleaser release --snapshot --clean
