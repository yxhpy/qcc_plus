#!/bin/bash
#
# Build script for @qccplus npm packages
# This script builds Go binaries and copies them to the npm package directories
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NPM_DIR="$PROJECT_ROOT/npm-packages/@qccplus"
VERSION="${1:-$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")}"
VERSION="${VERSION#v}"  # Remove 'v' prefix

echo "Building QCC Plus npm packages v$VERSION"
echo "================================================"

# Update package versions
for pkg in cli darwin-arm64 darwin-x64 linux-arm64 linux-x64 win32-x64; do
    if [ -f "$NPM_DIR/$pkg/package.json" ]; then
        # Use node to update version
        node -e "
            const fs = require('fs');
            const pkg = JSON.parse(fs.readFileSync('$NPM_DIR/$pkg/package.json'));
            pkg.version = '$VERSION';
            fs.writeFileSync('$NPM_DIR/$pkg/package.json', JSON.stringify(pkg, null, 2) + '\n');
        "
        echo "Updated $pkg to v$VERSION"
    fi
done

# Update optional dependencies version in cli package
node -e "
    const fs = require('fs');
    const pkgPath = '$NPM_DIR/cli/package.json';
    const pkg = JSON.parse(fs.readFileSync(pkgPath));
    if (pkg.optionalDependencies) {
        for (const dep of Object.keys(pkg.optionalDependencies)) {
            pkg.optionalDependencies[dep] = '$VERSION';
        }
    }
    fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + '\n');
"

echo ""
echo "Building frontend..."
echo "================================================"

cd "$PROJECT_ROOT/frontend"
npm ci
npm run build

# Copy frontend build to web/dist for Go embed
rm -rf "$PROJECT_ROOT/web/dist"
cp -R "$PROJECT_ROOT/frontend/dist" "$PROJECT_ROOT/web/dist"
echo "Frontend build copied to web/dist"

echo ""
echo "Building Go binaries..."
echo "================================================"

cd "$PROJECT_ROOT"

# Function to get npm arch from go arch
get_npm_arch() {
    case "$1" in
        amd64) echo "x64" ;;
        arm64) echo "arm64" ;;
        *) echo "$1" ;;
    esac
}

# Function to get npm os from go os
get_npm_os() {
    case "$1" in
        darwin) echo "darwin" ;;
        linux) echo "linux" ;;
        windows) echo "win32" ;;
        *) echo "$1" ;;
    esac
}

# Build for each target
build_target() {
    local GOOS=$1
    local GOARCH=$2

    local npm_os=$(get_npm_os "$GOOS")
    local npm_arch=$(get_npm_arch "$GOARCH")
    local pkg_dir="$NPM_DIR/${npm_os}-${npm_arch}"

    local binary_name="qccplus"
    if [ "$GOOS" = "windows" ]; then
        binary_name="qccplus.exe"
    fi

    echo "Building for $GOOS/$GOARCH -> $npm_os-$npm_arch"

    mkdir -p "$pkg_dir/bin"

    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "-s -w \
            -X 'qcc_plus/internal/version.Version=$VERSION' \
            -X 'qcc_plus/internal/version.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)' \
            -X 'qcc_plus/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
        -o "$pkg_dir/bin/$binary_name" \
        ./cmd/cccli

    chmod +x "$pkg_dir/bin/$binary_name" 2>/dev/null || true

    echo "  -> $pkg_dir/bin/$binary_name"
}

# Build all targets
build_target darwin arm64
build_target darwin amd64
build_target linux arm64
build_target linux amd64
build_target windows amd64

echo ""
echo "Build complete!"
echo "================================================"
echo ""
echo "Packages ready at: $NPM_DIR"
echo ""
echo "To publish:"
echo "  cd $NPM_DIR"
echo "  npm publish --access public ./darwin-arm64"
echo "  npm publish --access public ./darwin-x64"
echo "  npm publish --access public ./linux-arm64"
echo "  npm publish --access public ./linux-x64"
echo "  npm publish --access public ./win32-x64"
echo "  npm publish --access public ./cli"
echo ""
echo "Or use: npm run publish:all (after adding script)"
