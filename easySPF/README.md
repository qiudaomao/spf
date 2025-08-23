# EasySPF - Minimal C# Wrapper for SPF

EasySPF is the simplest way to integrate SPF (SSH Port Forwarding) into C# applications. Instead of creating a complex API, it simply calls the main function from `main_windows.go`, providing the exact same systray experience with minimal code.

## Overview

EasySPF provides a single function that starts the SPF application with full systray support:

```csharp
EasySPF.EasySPF.Run(); // Blocks until user quits from systray
```

This approach offers:
- ✅ **Minimal complexity** - Only one function call
- ✅ **Identical functionality** - Exact same behavior as `main_windows.go`
- ✅ **Full systray support** - All menus, groupings, and interactions
- ✅ **Easy integration** - Just add the library and call one function
- ✅ **No API learning** - Uses the same config.ini format

## Quick Start

### 1. Build the Library
```bash
./build_easy_spf.sh
```

### 2. Use in Your C# Application
```csharp
using EasySPF;

class Program
{
    static void Main()
    {
        try
        {
            // This will show SPF in the system tray and block until quit
            EasySPF.EasySPF.Run();
        }
        catch (SPFException ex)
        {
            Console.WriteLine($"Error: {ex.Message}");
        }
    }
}
```

### 3. Required Files
Place these files in your application directory:
- `config.ini` - Your SPF configuration
- `icon.ico` - Application icon (optional)
- `libeasyspf.dylib`/`libeasyspf.so`/`easyspf.dll` - Native library

## API Reference

### EasySPF.Run()
Starts the SPF application with systray support. This function will block until the user quits from the systray menu.

**Throws:** `SPFException` if the application fails to start (usually due to missing or invalid config.ini)

### EasySPF.RunAsync()
Async version that runs SPF in a background task while allowing your application to continue.

**Returns:** `Task` that completes when SPF exits

**Example:**
```csharp
// Start SPF in background
var spfTask = EasySPF.EasySPF.RunAsync();

// Your application continues running
DoOtherWork();

// Wait for SPF to exit (optional)
await spfTask;
```

## Configuration

Uses the same `config.ini` format as the original SPF:

```ini
[common]
debug = true

# SSH Server
[server1]
server = your-server.com
user = username
password = password
port = 22

# Local Port Forward
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

## Systray Features

When you call `EasySPF.Run()`, you get the complete systray experience:

### Menu Structure
- **Status display** showing current state
- **Server groupings** with configurations organized by SSH server
- **Configuration details** in the same format as the Go version:
  - `SectionName IP:Port r → l IP:Port` (remote forwarding)
  - `SectionName IP:Port l → r IP:Port` (local forwarding)  
  - `SectionName IP:Port l ← SOCKS5` (SOCKS5 proxy)
  - `SectionName IP:Port r → SOCKS5` (reverse SOCKS5)

### Interactive Features
- **Right-click** tray icon for context menu
- **Click configurations** to show detailed information in console
- **Quit menu item** to gracefully shut down
- **Connection status** and automatic retry on failures

## Comparison with Full SPF Library

| Feature | EasySPF | Full SPF Library |
|---------|---------|------------------|
| Lines of C# code | ~50 | ~500+ |
| API complexity | 1 function | Multiple classes/methods |
| Systray support | ✅ Full | ✅ Full |
| Programmatic control | ❌ None | ✅ Start/Stop/Status |
| Error handling | ❌ Basic | ✅ Detailed |
| Resource management | ❌ Automatic | ✅ Manual |
| Integration effort | ⚡ Minimal | 🔧 Moderate |

## Use Cases

### ✅ Perfect For:
- **Simple applications** that just need SSH forwarding
- **Legacy applications** requiring minimal changes
- **Quick prototypes** and demos
- **Applications where systray is the primary interface**

### ❌ Not Ideal For:
- **Applications needing programmatic control** (start/stop/status)
- **Applications requiring custom UI** instead of systray
- **Server applications** without user interaction
- **Applications needing detailed error handling**

## Integration Examples

### Console Application
```csharp
class Program
{
    static void Main()
    {
        Console.WriteLine("Starting SPF...");
        Console.WriteLine("Check your system tray!");
        
        EasySPF.EasySPF.Run(); // Blocks until quit
        
        Console.WriteLine("SPF stopped.");
    }
}
```

### Windows Forms Application
```csharp
public partial class MainForm : Form
{
    private async void startSPFButton_Click(object sender, EventArgs e)
    {
        startSPFButton.Enabled = false;
        
        try
        {
            // Run SPF in background while keeping form responsive
            await EasySPF.EasySPF.RunAsync();
        }
        catch (SPFException ex)
        {
            MessageBox.Show($"SPF Error: {ex.Message}");
        }
        finally
        {
            startSPFButton.Enabled = true;
        }
    }
}
```

### Service/Background Application
```csharp
class BackgroundService
{
    public async Task StartAsync()
    {
        // Start SPF in background
        var spfTask = EasySPF.EasySPF.RunAsync();
        
        // Continue with other service initialization
        InitializeOtherServices();
        
        // SPF runs independently with its own systray
        await spfTask;
    }
}
```

## Building and Deployment

### Build Requirements
- Go 1.21+ (for native library)
- .NET 6.0+ (for C# wrapper)

### Build Commands
```bash
# Build native library
./build_easy_spf.sh

# Build C# wrapper
cd easySPF
dotnet build
dotnet pack

# Build example
cd ../easySPF-example
dotnet build
dotnet run
```

### Deployment
Include these files with your application:
- Your application executable
- `EasySPF.dll` (or reference as NuGet package)
- `libeasyspf.dylib`/`libeasyspf.so`/`easyspf.dll`
- `config.ini` (your SSH configuration)
- `icon.ico` (optional, for systray icon)

## Platform Support

| Platform | Native Library | C# Wrapper | Status |
|----------|----------------|------------|---------|
| Windows  | `easyspf.dll` | ✅ | Full support |
| macOS    | `libeasyspf.dylib` | ✅ | Full support |  
| Linux    | `libeasyspf.so` | ✅ | Full support |

*Note: Systray functionality may vary by platform and desktop environment*

## Troubleshooting

### "Failed to run SPF application"
- Ensure `config.ini` exists and is properly formatted
- Check SSH server credentials and connectivity
- Verify native library is in the application directory

### "DLL not found" / "Library not found"
- Ensure the correct native library for your platform is present
- Check that the library file isn't corrupted
- On Linux, ensure required shared libraries are installed

### Systray not appearing
- Check if your desktop environment supports system tray
- Ensure `icon.ico` file is present (though not strictly required)
- Try running with admin/sudo privileges if needed

## License

Same license as the original SPF project.