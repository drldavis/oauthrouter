.PHONY: build test lint
build:
	go build -o bin/oauthrouter ./cmd/oauthrouter
test:
	go test -race ./...
lint:
	go vet ./...
