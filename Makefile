.PHONY: test cover build clean fmt vet lint staticcheck check

test:
	go test -v ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

check:
	go fmt ./...
	go vet ./...
	golangci-lint run
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...
	go test -v ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

build:
	go build -o ravenbot ./cmd/bot/main.go

clean:
	rm -f ravenbot coverage.out coverage.html
	rm -rf daily_logs
