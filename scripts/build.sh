#!/usr/bin/env bash
set -e

BINARY="knowurllm"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")

echo "Building $BINARY $VERSION..."

mkdir -p dist

GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w -X main.version=$VERSION" -o dist/${BINARY}-linux-amd64    ./cmd/knowurllm/
GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w -X main.version=$VERSION" -o dist/${BINARY}-darwin-amd64   ./cmd/knowurllm/
GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w -X main.version=$VERSION" -o dist/${BINARY}-darwin-arm64   ./cmd/knowurllm/
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.version=$VERSION" -o dist/${BINARY}-windows-amd64.exe ./cmd/knowurllm/

echo "Done. Binaries in dist/"
