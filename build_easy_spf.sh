#!/bin/bash

# Build EasySPF minimal library

echo "Building EasySPF minimal library..."

# Build for current platform
echo "Building for $(uname -s)..."
case "$(uname -s)" in
    Darwin)
        # macOS
        go build -buildmode=c-shared -o libeasyspf.dylib easy_spf.go
        ;;
    Linux)
        # Linux
        go build -buildmode=c-shared -o libeasyspf.so easy_spf.go
        ;;
    MINGW*|CYGWIN*|MSYS*)
        # Windows
        go build -buildmode=c-shared -o easyspf.dll easy_spf.go
        ;;
    *)
        echo "Unsupported platform: $(uname -s)"
        exit 1
        ;;
esac

echo "EasySPF library built successfully!"
echo "Header file: easy_spf.h"

# Show the created files
ls -la libeasyspf* easyspf* easy_spf.h 2>/dev/null || true