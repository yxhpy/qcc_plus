#!/bin/bash
set -e

echo "Building React frontend..."
cd frontend
npm run build
cd ..

echo "Copying build to web/dist..."
rm -rf web/dist
cp -R frontend/dist web/dist

echo "Frontend build complete!"
echo "Next steps:"
echo "  1. Run: go build -o cccli_bin ./cmd/cccli"
echo "  2. Start server: ./cccli_bin proxy"
