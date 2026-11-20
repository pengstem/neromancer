#!/usr/bin/env bash
set -euo pipefail

echo "==> [1/3] Running go vet ./..."
go vet ./...

echo "==> [2/3] Running go tool mygo vet (checking ui.Element & *ui.Context lifetimes)..."
go tool mygo vet

echo "==> [3/3] Running go test -race ./..."
go test -race ./...

echo "✅ All MyGo static, lifetime, and race checks passed."
