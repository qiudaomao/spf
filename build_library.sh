#!/bin/bash

# Build SPF as a shared library

echo "Building SPF shared library..."

# Build for current platform
echo "Building for $(uname -s)..."
case "$(uname -s)" in
    Darwin)
        # macOS
        go build -buildmode=c-shared -o libspf.dylib spf.go
        ;;
    Linux)
        # Linux
        go build -buildmode=c-shared -o libspf.so spf.go
        ;;
    MINGW*|CYGWIN*|MSYS*)
        # Windows
        go build -buildmode=c-shared -o spf.dll spf.go
        ;;
    *)
        echo "Unsupported platform: $(uname -s)"
        exit 1
        ;;
esac

echo "Library built successfully!"
echo "Header file: spf.h"

# Show the created files
ls -la libspf* spf.h 2>/dev/null || true