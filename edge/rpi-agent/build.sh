#!/usr/bin/env bash
set -e
ARCH=${1:-arm64}
if [[ $ARCH == "arm64" ]]; then
  export GOOS=linux GOARCH=arm64
elif [[ $ARCH == "armv7" ]]; then
  export GOOS=linux GOARCH=arm GOARM=7
else
  echo "arch must be arm64 or armv7"; exit 1
fi
go mod tidy
go build -o bin/axcp-agent ./cmd/agent
echo "Built bin/axcp-agent for $GOARCH"
