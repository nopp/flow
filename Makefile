.PHONY: run build test tidy

run:
	go run ./cmd/cicd

build:
	go build -o bin/cicd ./cmd/cicd

test:
	go test ./...

tidy:
	go mod tidy
