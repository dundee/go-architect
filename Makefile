.PHONY: web-install web-build build install run test lint vet vulncheck install-dependencies

web-install:
	cd web && npm install

web-build:
	cd web && npm run build

build:
	go build .

install:
	go install .

run:
	go run . -path .

test:
	go test ./...

lint:
	golangci-lint run

vet:
	go vet ./...

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

install-dependencies:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
