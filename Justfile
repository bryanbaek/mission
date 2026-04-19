set dotenv-load := true

default:
    @just --list

# Go

build:
    go build ./...

test:
    go test ./...

lint:
    golangci-lint run ./...

vet:
    go vet ./...

run-control-plane:
    go run ./cmd/control-plane

run-edge-agent:
    go run ./cmd/edge-agent

tidy:
    go mod tidy

# Proto / Connect-RPC

generate:
    buf generate
    go mod tidy

# Frontend

web-install:
    cd web && npm install

web-dev:
    cd web && npm run dev

web-build:
    cd web && npm run build

web-typecheck:
    cd web && npm run typecheck

# Docker compose

up:
    docker compose up --build

down:
    docker compose down

logs:
    docker compose logs -f
