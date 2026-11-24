.PHONY: web-install web-build build install run test lint vet install-dependencies

web-install:
	cd web && npm install

web-build:
	cd web && npm run build

build:
	go build -o go-architect ./cmd/go-architect

install:
	go install ./cmd/go-architect

run:
	go run ./cmd/go-architect -path .

test:
	go test ./...

lint:
	golangci-lint run

vet:
	go vet ./...

install-dependencies:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest