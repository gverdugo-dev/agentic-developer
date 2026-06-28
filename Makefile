run: build
	@go run ./cmd/agentic-dev/main.go

build:
	@go build -o ./bin/agentic-dev ./cmd/agentic-dev
