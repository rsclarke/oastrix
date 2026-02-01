.PHONY: build test lint clean

build:
	go build -o oastrix ./cmd/oastrix

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -f oastrix
