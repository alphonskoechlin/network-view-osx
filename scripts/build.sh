#!/bin/bash
set -e

# Cross-compile build script for macOS, Linux, Windows
# Supports: amd64, arm64

VERSION="${1:-dev}"
OUTPUT_DIR="${2:-.}"

BINARY_NAME="network-view-osx"

echo "🔨 Building $BINARY_NAME v$VERSION"
echo "📁 Output directory: $OUTPUT_DIR"

# Array of platforms and architectures
declare -a PLATFORMS=(
    "darwin:amd64"
    "darwin:arm64"
    "linux:amd64"
    "linux:arm64"
    "windows:amd64"
    "windows:arm64"
)

# Create output directory
mkdir -p "$OUTPUT_DIR"

cd backend

for platform in "${PLATFORMS[@]}"; do
    IFS=':' read -r OS ARCH <<< "$platform"
    
    OUTPUT_FILE="$OUTPUT_DIR/${BINARY_NAME}-${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        OUTPUT_FILE="${OUTPUT_FILE}.exe"
    fi
    
    echo "📦 Building $OS/$ARCH → $OUTPUT_FILE"
    
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build \
        -ldflags="-w -s -X main.Version=$VERSION" \
        -o "$OUTPUT_FILE" \
        .
    
    # Strip binary on Unix-like systems
    if [ "$OS" != "windows" ]; then
        strip "$OUTPUT_FILE" 2>/dev/null || true
    fi
done

echo "✓ All builds complete!"
echo ""
echo "Binaries:"
ls -lh "$OUTPUT_DIR"/${BINARY_NAME}-*
