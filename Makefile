.PHONY: build run test

build:
	go build -o bin/gtracer main.go

run:
	go run main.go

test:
	go test ./...