run: build
	@go run ./cmd/adev/main.go

build:
	@go build -o ./bin/adev ./cmd/adev

install:
	@go install ./cmd/adev
