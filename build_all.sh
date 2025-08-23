#!/bin/bash

# Build script for SPF library and all example applications

set -e

echo "Building SPF Complete Solution"
echo "=============================="

# 1. Build the native library
echo "Step 1: Building native library..."
./build_library.sh

# 2. Build C# wrapper (if dotnet is available)
if command -v dotnet &> /dev/null; then
    echo "Step 2: Building C# wrapper..."
    cd csharp
    dotnet build --configuration Release
    dotnet pack --configuration Release
    cd ..
    
    echo "Step 3: Building systray example..."
    cd example
    dotnet build --configuration Release
    cd ..
    
    echo "Step 4: Building console example..."
    cd example-console
    dotnet build --configuration Release
    cd ..
    
    echo "✅ All components built successfully!"
    echo ""
    echo "Built components:"
    echo "- Native library: libspf.dylib/libspf.so/spf.dll"
    echo "- C# wrapper: csharp/bin/Release/"
    echo "- Systray example: example/bin/Release/"
    echo "- Console example: example-console/bin/Release/"
    echo ""
    echo "To run the examples:"
    echo "- Windows Systray: cd example && dotnet run"
    echo "- Console: cd example-console && dotnet run"
else
    echo "Step 2-4: Skipping C# builds (.NET SDK not found)"
    echo "✅ Native library built successfully!"
    echo ""
    echo "Built components:"
    echo "- Native library: libspf.dylib/libspf.so/spf.dll"
    echo "- C header: spf.h"
    echo ""
    echo "To build C# components, install .NET 6.0+ SDK and run:"
    echo "- cd csharp && dotnet build"
    echo "- cd example && dotnet build"
    echo "- cd example-console && dotnet build"
fi

echo ""
echo "📚 Documentation:"
echo "- Library usage: README_LIBRARY.md"
echo "- Original project: readme.md"