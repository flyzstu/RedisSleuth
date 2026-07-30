.PHONY: test vet build check

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -o bin/redissleuth .

check: test vet build
