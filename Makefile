.PHONY: build test clean

build:
	go build -o oastrix ./cmd/oastrix

test:
	go test ./...

clean:
	rm -f oastrix
