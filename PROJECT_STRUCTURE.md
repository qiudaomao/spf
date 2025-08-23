# SPF Project Structure

This document outlines the complete structure of the SPF (SSH Port Forwarding) project after conversion to a C# library.

## Project Layout

```
spf/
├── README_LIBRARY.md          # Library documentation
├── PROJECT_STRUCTURE.md       # This file
├── build_all.sh              # Complete build script
├── build_library.sh          # Native library build script
│
├── Original Go Application/
│   ├── main.go               # Original Unix/Linux version
│   ├── main_windows.go       # Original Windows systray version
│   ├── go.mod               # Go module definition
│   ├── config.ini           # Example configuration
│   ├── icon.ico            # Application icon
│   └── release/            # Compiled binaries
│
├── Library Components/
│   ├── spf.go              # Go library with C exports
│   ├── spf.h               # C header file
│   ├── libspf.dylib        # macOS shared library
│   ├── libspf.so           # Linux shared library  
│   └── spf.dll             # Windows shared library
│
├── C# Wrapper/
│   └── csharp/
│       ├── SPF.csproj      # .NET project file
│       └── SPF.cs          # C# wrapper with P/Invoke
│
├── Systray Example (Windows)/
│   └── example/
│       ├── Example.csproj  # Windows Forms project
│       ├── Program.cs      # Systray application
│       └── icon.ico        # Application icon
│
├── Console Example (Cross-Platform)/
│   └── example-console/
│       ├── SPFConsole.csproj # Console project
│       └── ProgramConsole.cs # Console application
│
├── EasySPF (Minimal Solution)/
│   ├── easySPF/
│   │   ├── EasySPF.csproj    # Minimal C# wrapper project
│   │   ├── EasySPF.cs        # Single function wrapper
│   │   └── README.md         # EasySPF documentation
│   ├── easySPF-example/
│   │   ├── EasySPFExample.csproj # Example project
│   │   └── Program.cs        # Simple usage example
│   ├── easy_spf.go           # Minimal Go library (exports main only)
│   ├── easy_spf.h            # Minimal C header
│   ├── libeasyspf.*          # Minimal shared libraries
│   └── build_easy_spf.sh     # Minimal library build script
│
└── Build Scripts/
    ├── build_complete.sh     # Build everything
    ├── build_all.sh          # Build full solution
    └── build_easy_spf.sh     # Build minimal solution
```

## Component Overview

### 1. Native Library (Go → C)
- **Input**: Go source code with SSH forwarding logic
- **Output**: Shared library (.dylib/.so/.dll) with C-compatible exports
- **API**: Instance-based C functions for create/start/stop/destroy operations

### 2. C# Wrapper Library
- **Input**: Native library + C header
- **Output**: .NET package with managed API
- **Features**: P/Invoke bindings, resource management, exceptions

### 3. Windows Systray Application
- **Technology**: WinForms + NotifyIcon (WPF TaskbarIcon)
- **Features**: System tray integration, context menus, status notifications
- **Compatibility**: Windows-specific (matches main_windows.go functionality)

### 4. Console Application  
- **Technology**: .NET Console Application
- **Features**: Interactive command interface, cross-platform compatibility
- **Compatibility**: Windows, macOS, Linux

### 5. EasySPF (Minimal Solution)
- **Technology**: Single C export + minimal C# wrapper
- **Features**: Just calls main_windows.go function directly
- **API**: One function: `EasySPF.Run()`
- **Use Case**: Simplest possible integration

## Build Process

### Prerequisites
- Go 1.21+ (for native library)
- .NET 6.0+ SDK (for C# components)

### Build Commands

1. **Everything**: `./build_complete.sh`
2. **Full solution**: `./build_all.sh`
3. **Minimal solution**: `./build_easy_spf.sh`
4. **Native libraries only**: `./build_library.sh`  
5. **C# components only**: `cd csharp && dotnet build`

## Usage Scenarios

### 1. Library Integration
```csharp
using SPF;
using var client = new SPFClient("config.ini");
client.Start();
// ... application logic ...
client.Stop();
```

### 2. Systray Application
- Double-click to run
- Lives in system tray
- Right-click for menu
- Matches Go systray behavior

### 3. Console Application
- Interactive command interface
- Cross-platform compatibility
- Detailed status and control

### 4. EasySPF (Minimal)
```csharp
// Simplest possible integration
EasySPF.EasySPF.Run(); // Shows systray, blocks until quit
```

## Configuration

Both applications use the same INI format as the original Go version:

```ini
[common]
debug = true

[server1]
server = ssh-host.com
user = username
password = password

[forward-web]
server = server1
direction = local
localIP = 127.0.0.1
localPort = 8080
remoteIP = 127.0.0.1
remotePort = 80
```

## Key Features Preserved

### From main.go
- ✅ SSH connection management
- ✅ Local/remote port forwarding
- ✅ SOCKS5 proxy support
- ✅ Reverse SOCKS5 proxy
- ✅ Connection retry logic
- ✅ Configuration file parsing

### From main_windows.go
- ✅ System tray integration
- ✅ Server-grouped menu structure  
- ✅ Configuration status display
- ✅ Clickable menu items with details
- ✅ Connection status indicators
- ✅ Graceful shutdown

## Distribution

### For End Users
- Copy native library (libspf.*/spf.dll) with your application
- Include icon.ico for systray applications
- Provide config.ini template

### For Developers
- Reference SPF.csproj in your projects
- Use NuGet package when available
- Include native libraries in output directory

## Solution Comparison

| Feature | EasySPF | Full SPF Library |
|---------|---------|------------------|
| Lines of C# code | ~50 | ~500+ |
| API complexity | 1 function | Multiple classes |
| Systray support | ✅ Full | ✅ Full |
| Programmatic control | ❌ None | ✅ Complete |
| Integration effort | ⚡ Minimal | 🔧 Moderate |
| Best for | Simple apps | Complex apps |

## Platform Support

| Component | Windows | macOS | Linux |
|-----------|---------|--------|--------|
| Native Library | ✅ | ✅ | ✅ |
| C# Wrapper | ✅ | ✅ | ✅ |
| Console Example | ✅ | ✅ | ✅ |
| Systray Example | ✅ | ❌ | ❌ |
| EasySPF | ✅ | ✅ | ✅ |

*Note: Systray is Windows-specific by design to match main_windows.go behavior*