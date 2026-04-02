#!/bin/bash
set -e

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "Building React frontend..."
cd "$ROOT_DIR/frontend"
npm run build
cd "$ROOT_DIR"

echo "Copying build to web/dist..."
rm -rf "$ROOT_DIR/web/dist"
cp -R "$ROOT_DIR/frontend/dist" "$ROOT_DIR/web/dist"

echo "Frontend build complete!"
echo "Next steps:"
echo "  1. Run: go build -o cccli_bin ./cmd/cccli"
echo "  2. Start server: ./cccli_bin proxy"
