.PHONY: build test clean lint

build:
	go build -o bin/kage ./cmd/kage

test:
	go test ./...

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...
