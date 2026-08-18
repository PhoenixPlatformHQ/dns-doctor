#!/bin/bash
set -e
export PATH=/home/fserdonio/go/bin:/usr/local/bin:/usr/bin:/bin
export GONOSUMDB="*"
export GONOSUMCHECK="*"
export GOFLAGS=""
export GIT_TERMINAL_PROMPT=0
export GOPROXY="https://proxy.golang.org,direct"
export GOPATH=/tmp/gopath_dnd
REPO=/mnt/c/dev/PhoenixPlatform/tools/dns-doctor
cd "$REPO"

echo "=== go version ==="
go version

echo "=== go mod tidy (regenerate go.sum) ==="
go mod tidy

echo "=== go vet ==="
go vet ./...

echo "=== go test ==="
go test ./... -v -count=1

echo "=== build linux/amd64 ==="
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags "-s -w -X main.version=0.1.0" \
  -o /tmp/kubectl-dns_doctor-linux-amd64 ./cmd/dns-doctor

echo "=== binary info ==="
ls -lh /tmp/kubectl-dns_doctor-linux-amd64
file /tmp/kubectl-dns_doctor-linux-amd64

echo "=== cross-compile linux/arm64 ==="
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -ldflags "-s -w -X main.version=0.1.0" \
  -o /tmp/kubectl-dns_doctor-linux-arm64 ./cmd/dns-doctor
ls -lh /tmp/kubectl-dns_doctor-linux-arm64

echo "=== cross-compile darwin/amd64 ==="
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
  -ldflags "-s -w -X main.version=0.1.0" \
  -o /tmp/kubectl-dns_doctor-darwin-amd64 ./cmd/dns-doctor
ls -lh /tmp/kubectl-dns_doctor-darwin-amd64

echo "=== cross-compile darwin/arm64 ==="
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
  -ldflags "-s -w -X main.version=0.1.0" \
  -o /tmp/kubectl-dns_doctor-darwin-arm64 ./cmd/dns-doctor
ls -lh /tmp/kubectl-dns_doctor-darwin-arm64

echo "=== cross-compile windows/amd64 ==="
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
  -ldflags "-s -w -X main.version=0.1.0" \
  -o /tmp/kubectl-dns_doctor-windows-amd64.exe ./cmd/dns-doctor
ls -lh /tmp/kubectl-dns_doctor-windows-amd64.exe

echo "=== DONE ==="
