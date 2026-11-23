.PHONY: web-install web-build build install run test

web-install:
	cd web && npm install

web-build:
	cd web && npm run build

build:
	go build -o go-architect ./cmd/web

install:
	go install ./cmd/web

run:
	go run ./cmd/web -path .

test:
	go test ./...
