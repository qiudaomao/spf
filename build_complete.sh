#!/bin/bash

# Complete build script for all SPF variants

set -e

echo "Building Complete SPF Solution"
echo "=============================="
echo ""

# 1. Build original Go binaries
echo "Step 1: Building original Go applications..."
echo "  - Building Unix/Linux version (main.go)..."
go build -o spf_unix main.go

if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
    echo "  - Building Windows systray version (main_windows.go)..."
    go build -o spf_windows.exe main_windows.go
else
    echo "  - Skipping Windows build (not on Windows platform)"
fi

echo ""

# 2. Build full SPF library
echo "Step 2: Building full SPF library..."
./build_library.sh

echo ""

# 3. Build minimal EasySPF library  
echo "Step 3: Building minimal EasySPF library..."
./build_easy_spf.sh

echo ""

# 4. Build C# components (if dotnet is available)
if command -v dotnet &> /dev/null; then
    echo "Step 4: Building C# components..."
    
    echo "  - Building full SPF C# wrapper..."
    cd csharp
    dotnet build --configuration Release
    dotnet pack --configuration Release
    cd ..
    
    echo "  - Building EasySPF C# wrapper..."
    cd easySPF
    dotnet build --configuration Release
    dotnet pack --configuration Release
    cd ..
    
    echo "  - Building systray example..."
    cd example
    dotnet build --configuration Release
    cd ..
    
    echo "  - Building console example..."
    cd example-console
    dotnet build --configuration Release
    cd ..
    
    echo "  - Building EasySPF example..."
    cd easySPF-example
    dotnet build --configuration Release
    cd ..
    
    echo ""
    echo "✅ All components built successfully!"
    echo ""
    
    echo "📦 Built Components:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Original Go Applications:"
    echo "  - spf_unix                    # Unix/Linux console version"
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
        echo "  - spf_windows.exe             # Windows systray version"
    fi
    echo ""
    echo "Libraries:"
    echo "  - libspf.*/spf.dll            # Full API library (create/start/stop/destroy)"
    echo "  - libeasyspf.*/easyspf.dll    # Minimal library (single run function)"
    echo ""
    echo "C# Wrappers:"
    echo "  - csharp/bin/Release/         # Full SPF API wrapper"
    echo "  - easySPF/bin/Release/        # Minimal EasySPF wrapper"
    echo ""
    echo "Example Applications:"
    echo "  - example/bin/Release/        # Windows systray app (full API)"
    echo "  - example-console/bin/Release/# Cross-platform console app"
    echo "  - easySPF-example/bin/Release/# Minimal wrapper example"
    echo ""
    
    echo "🚀 Usage Examples:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Original Go applications:"
    echo "  ./spf_unix                    # Run console version"
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
        echo "  ./spf_windows.exe             # Run Windows systray version"
    fi
    echo ""
    echo "C# applications:"
    echo "  cd example && dotnet run              # Full API systray app"
    echo "  cd example-console && dotnet run     # Console interface"
    echo "  cd easySPF-example && dotnet run     # Minimal wrapper"
    echo ""
    
    echo "📚 Documentation:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  - README_LIBRARY.md           # Full library documentation"
    echo "  - easySPF/README.md           # Minimal wrapper documentation"
    echo "  - PROJECT_STRUCTURE.md        # Complete project overview"
    echo ""
    
else
    echo "Step 4: Skipping C# builds (.NET SDK not found)"
    echo ""
    echo "✅ Go components built successfully!"
    echo ""
    
    echo "📦 Built Components:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Original Go Applications:"
    echo "  - spf_unix                    # Unix/Linux console version"
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
        echo "  - spf_windows.exe             # Windows systray version"
    fi
    echo ""
    echo "Libraries:"
    echo "  - libspf.*/spf.dll            # Full API library"
    echo "  - libeasyspf.*/easyspf.dll    # Minimal library"
    echo "  - spf.h, easy_spf.h           # C header files"
    echo ""
    
    echo "To build C# components, install .NET 6.0+ SDK and run:"
    echo "  dotnet build csharp/SPF.csproj"
    echo "  dotnet build easySPF/EasySPF.csproj"
    echo ""
fi

echo "🎯 Choose Your Integration Approach:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "1. 🚀 EasySPF (Minimal)    - Just call EasySPF.Run()"
echo "2. 🔧 Full SPF Library     - Complete programmatic control"  
echo "3. 📱 Direct Go Usage      - Use original binaries"
echo ""