# SPF C# Integration Guide

This guide helps you choose the right approach for integrating SPF (SSH Port Forwarding) into your C# application.

## Quick Decision Tree

```
Do you need programmatic control (start/stop/status)?
├─ YES → Use Full SPF Library
└─ NO → Use EasySPF (Minimal)
```

## Option 1: EasySPF (Minimal) 🚀

**Perfect for**: Simple applications that just need SSH forwarding

### Code Required
```csharp
using EasySPF;

// That's it - one function call!
EasySPF.EasySPF.Run(); // Shows systray, blocks until user quits
```

### Pros
- ✅ **Minimal code** - Just one function call
- ✅ **Identical to Go version** - Exact same systray behavior
- ✅ **No learning curve** - Uses same config.ini format
- ✅ **Easy deployment** - Just include native library

### Cons
- ❌ **No programmatic control** - Can't start/stop from code
- ❌ **Blocks main thread** - Use `RunAsync()` if needed
- ❌ **Basic error handling** - Limited exception information

### Files Needed
```
YourApp.exe
EasySPF.dll
libeasyspf.dylib    (macOS)
libeasyspf.so       (Linux) 
easyspf.dll         (Windows)
config.ini
icon.ico            (optional)
```

### Build Commands
```bash
./build_easy_spf.sh
cd easySPF && dotnet build
```

---

## Option 2: Full SPF Library 🔧

**Perfect for**: Applications needing full control over SSH forwarding

### Code Required
```csharp
using SPF;

using var spf = new SPFClient("config.ini");
spf.Start();

// Your application logic here
Console.WriteLine($"Status: {spf.IsRunning}");

spf.Stop();
// Dispose called automatically
```

### Pros
- ✅ **Full programmatic control** - Start, stop, status from code
- ✅ **Resource management** - Explicit lifecycle control
- ✅ **Detailed error handling** - Rich exception information
- ✅ **Multiple instances** - Can run several SPF instances

### Cons
- ❌ **More complex** - Need to understand the API
- ❌ **More code** - Requires proper resource management
- ❌ **Manual systray** - Need to implement your own if wanted

### Files Needed
```
YourApp.exe
SPF.dll
libspf.dylib        (macOS)
libspf.so           (Linux)
spf.dll             (Windows)
config.ini
```

### Build Commands
```bash
./build_library.sh
cd csharp && dotnet build
```

---

## Option 3: Direct Go Usage 📱

**Perfect for**: When you just want to ship the original Go binaries

### Code Required
```csharp
using System.Diagnostics;

// Launch the original Go application
var process = Process.Start("spf_windows.exe");
process.WaitForExit();
```

### Pros
- ✅ **No wrapper needed** - Direct use of Go binaries
- ✅ **Proven stable** - Original, tested implementation
- ✅ **Smallest footprint** - No additional libraries

### Cons
- ❌ **No integration** - Separate process, no control
- ❌ **Platform-specific** - Different binaries per platform
- ❌ **No error handling** - Can't catch Go application errors

### Files Needed
```
YourApp.exe
spf_windows.exe     (Windows)
spf_unix            (Unix/Linux)
config.ini
icon.ico            (for Windows version)
```

### Build Commands
```bash
go build -o spf_unix main.go
go build -o spf_windows.exe main_windows.go  # On Windows
```

---

## Comparison Matrix

| Feature | EasySPF | Full Library | Direct Go |
|---------|---------|-------------|-----------|
| **Integration Effort** | ⚡ Minimal | 🔧 Moderate | 🛠️ None |
| **Code Lines** | ~5 | ~20 | ~3 |
| **Programmatic Control** | ❌ | ✅ | ❌ |
| **Systray Support** | ✅ | ✅* | ✅ |
| **Error Handling** | 🟡 Basic | ✅ Full | ❌ |
| **Resource Management** | ✅ Auto | 🟡 Manual | ❌ |
| **Multiple Instances** | ❌ | ✅ | ✅ |
| **Cross-Platform** | ✅ | ✅ | 🟡** |
| **File Size** | Medium | Large | Small |

\* Requires implementing your own systray  
\** Need different binaries per platform

---

## Example Applications

### EasySPF Example
```csharp
class Program
{
    static async Task Main()
    {
        Console.WriteLine("Starting SPF - check your system tray!");
        
        try
        {
            // This will show the systray and block until quit
            await EasySPF.EasySPF.RunAsync();
        }
        catch (SPFException ex)
        {
            Console.WriteLine($"Error: {ex.Message}");
        }
        
        Console.WriteLine("SPF stopped.");
    }
}
```

### Full Library Example
```csharp
class Program
{
    static async Task Main()
    {
        using var spf = new SPFClient("config.ini");
        
        spf.Start();
        Console.WriteLine("SPF started - press any key to stop");
        
        // Your application can do other work here
        await SomeOtherWork();
        
        Console.ReadKey();
        spf.Stop();
        Console.WriteLine("SPF stopped.");
    }
}
```

### Direct Go Example
```csharp
class Program
{
    static void Main()
    {
        var startInfo = new ProcessStartInfo
        {
            FileName = "spf_windows.exe",
            WorkingDirectory = AppDomain.CurrentDomain.BaseDirectory,
            UseShellExecute = false
        };
        
        using var process = Process.Start(startInfo);
        Console.WriteLine("SPF started - it will show in your system tray");
        Console.WriteLine("Use the tray menu to quit SPF");
        
        // Your application continues independently
        DoOtherWork();
    }
}
```

---

## Configuration

All approaches use the same `config.ini` format:

```ini
[common]
debug = true

# SSH Server
[server1]
server = your-server.com
user = username
password = password
port = 22

# Port Forwarding
[forward-web]
server = server1
direction = local
localIP = 127.0.0.1
localPort = 8080
remoteIP = 127.0.0.1
remotePort = 80

# SOCKS5 Proxy  
[socks5-proxy]
server = server1
direction = socks5
localIP = 127.0.0.1
localPort = 1080
```

---

## Deployment Checklist

### EasySPF Deployment
- [ ] `YourApp.exe`
- [ ] `EasySPF.dll`
- [ ] `libeasyspf.dylib`/`libeasyspf.so`/`easyspf.dll`
- [ ] `config.ini` (configured with your servers)
- [ ] `icon.ico` (optional, for systray)

### Full Library Deployment
- [ ] `YourApp.exe`
- [ ] `SPF.dll`
- [ ] `libspf.dylib`/`libspf.so`/`spf.dll`
- [ ] `config.ini` (configured with your servers)

### Direct Go Deployment
- [ ] `YourApp.exe`
- [ ] `spf_windows.exe`/`spf_unix`
- [ ] `config.ini` (configured with your servers)
- [ ] `icon.ico` (for Windows version)

---

## Getting Started

1. **Clone and build**:
   ```bash
   git clone <repository>
   cd spf
   ./build_complete.sh
   ```

2. **Choose your approach** based on the decision tree above

3. **Copy the example** that matches your choice:
   - EasySPF: `easySPF-example/`
   - Full Library: `example/` or `example-console/`
   - Direct Go: Use `Process.Start()`

4. **Configure** your `config.ini` with your SSH servers

5. **Deploy** with the files listed in the deployment checklist

---

## Support

- 📖 **EasySPF**: See `easySPF/README.md`
- 📖 **Full Library**: See `README_LIBRARY.md`
- 📖 **Project Overview**: See `PROJECT_STRUCTURE.md`
- 🐛 **Issues**: Check existing issues or create new ones
- 💬 **Questions**: Use discussions for general questions

**Happy forwarding!** 🚀