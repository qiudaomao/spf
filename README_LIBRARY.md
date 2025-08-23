# SPF Library - SSH Port Forwarding for C#

SPF is a library that provides SSH port forwarding capabilities for C# applications. It supports local port forwarding, remote port forwarding, SOCKS5 proxy, and reverse SOCKS5 proxy.

## Features

- **Local Port Forwarding**: Forward connections from a local port to a remote server through an SSH tunnel
- **Remote Port Forwarding**: Forward connections from a remote port back to your local machine
- **SOCKS5 Proxy**: Create a SOCKS5 proxy server that routes traffic through an SSH tunnel
- **Reverse SOCKS5 Proxy**: Allow a remote server to access the internet through your local connection
- **Connection Reuse**: Efficient connection management with shared SSH connections
- **Authentication**: Support for password authentication and SOCKS5 proxy authentication
- **Cross-Platform**: Works on Windows, macOS, and Linux

## Building the Library

### Prerequisites

- Go 1.21 or later
- .NET 6.0 or later (for C# wrapper and examples)

### Build Steps

1. **Build the native library:**
   ```bash
   ./build_library.sh
   ```

   This will create:
   - `libspf.dylib` (macOS)
   - `libspf.so` (Linux)
   - `spf.dll` (Windows)
   - `spf.h` (C header file)

2. **Build the C# wrapper:**
   ```bash
   cd csharp
   dotnet build
   dotnet pack
   ```

3. **Build and run the examples:**
   
   **Windows Systray Version:**
   ```bash
   cd example
   dotnet run
   ```
   
   **Cross-Platform Console Version:**
   ```bash
   cd example-console
   dotnet run
   ```

## Usage

SPF provides two example applications:

### 1. Windows Systray Application (example/)
A Windows-specific GUI application that runs in the system tray, providing:
- **System tray icon** with tooltip showing current status
- **Context menu** with server groupings (same as Go version)
- **Configuration details** when clicking on menu items
- **Status window** accessible via double-click or menu
- **Balloon notifications** for status changes
- **Quit confirmation** dialog

### 2. Cross-Platform Console Application (example-console/)
A console-based application that works on all platforms:
- **Interactive command interface** with status display
- **Configuration listing** grouped by server
- **Real-time control** (start/stop/restart)
- **Detailed information** for each configuration
- **Cross-platform compatibility** (Windows, macOS, Linux)

### Configuration File

Create a configuration file (e.g., `config.ini`) with your SSH servers and forwarding rules:

```ini
[common]
debug = true

# SSH Server Configuration
[server1]
server = your-ssh-server.com
user = your-username
password = your-password
port = 22

# Local Port Forwarding - Forward local port 8080 to remote port 80
[forward-web]
server = server1
direction = local
localIP = 127.0.0.1
localPort = 8080
remoteIP = 127.0.0.1
remotePort = 80

# SOCKS5 Proxy - Create SOCKS5 proxy on port 1080
[socks5-proxy]
server = server1
direction = socks5
localIP = 127.0.0.1
localPort = 1080
```

### Systray Features

The Windows systray application replicates the functionality of `main_windows.go`:

#### Menu Structure
- **Status display** at the top showing current state
- **Server grouping** with configurations organized by SSH server
- **Configuration details** showing direction and port information:
  - `remote`: `RemoteIP:Port r → l LocalIP:Port`
  - `local`: `LocalIP:Port l → r RemoteIP:Port`
  - `socks5`: `LocalIP:Port l ← SOCKS5`
  - `reverse-socks5`: `RemoteIP:Port r → SOCKS5`

#### Interactive Features
- **Double-click** tray icon to show status window
- **Right-click** for context menu
- **Configuration clicks** show detailed information
- **Automatic notifications** for status changes
- **Graceful shutdown** with confirmation

### C# Code Example

```csharp
using SPF;

// Create and use SPF instance
using (var spf = new SPFClient("config.ini"))
{
    // Start port forwarding
    spf.Start();
    
    Console.WriteLine($"Running: {spf.IsRunning}");
    
    // Your application logic here...
    
    // Stop port forwarding
    spf.Stop();
    
    // SPF instance is automatically disposed
}
```

### Advanced Usage

```csharp
try
{
    var spf = new SPFClient("config.ini");
    
    // Start forwarding
    spf.Start();
    
    // Monitor status
    while (spf.IsRunning)
    {
        // Do work...
        Thread.Sleep(1000);
    }
}
catch (SPFException ex)
{
    Console.WriteLine($"SPF Error: {ex.Message}");
}
```

## Configuration Options

### Server Configuration
- `server`: SSH server hostname or IP
- `user`: SSH username
- `password`: SSH password
- `port`: SSH port (default: 22)

### Forward Configuration
- `server`: Reference to server configuration
- `direction`: Type of forwarding (`local`, `remote`, `socks5`, `reverse-socks5`)
- `localIP`: Local IP address to bind to
- `localPort`: Local port to bind to
- `remoteIP`: Remote IP address (for local/remote forwarding)
- `remotePort`: Remote port (for local/remote forwarding)
- `socks5User`: SOCKS5 username (optional)
- `socks5Pass`: SOCKS5 password (optional)

### Common Configuration
- `debug`: Enable debug logging (true/false)

## API Reference

### SPFClient Class

#### Constructor
- `SPFClient(string configPath)`: Create a new SPF instance with the specified configuration file

#### Methods
- `void Start()`: Start port forwarding
- `void Stop()`: Stop port forwarding
- `void Dispose()`: Clean up resources

#### Properties
- `bool IsRunning`: Check if port forwarding is active

#### Exceptions
- `SPFException`: Thrown when SPF operations fail

## Deployment

When deploying your C# application:

1. **Include the native library** in your application's output directory:
   - Windows: `spf.dll`
   - macOS: `libspf.dylib`
   - Linux: `libspf.so`

2. **NuGet Package**: You can create a NuGet package that includes the native libraries:
   ```bash
   cd csharp
   dotnet pack
   ```

3. **Runtime Dependencies**: Ensure the target system has the necessary runtime dependencies.

## Security Considerations

- Store SSH credentials securely (consider using environment variables or secure configuration)
- Use strong SSH passwords or key-based authentication
- Limit access to configuration files
- Consider using SSH key authentication instead of passwords
- Be cautious with SOCKS5 proxy configurations in production

## Troubleshooting

### Common Issues

1. **Library not found**: Ensure the native library is in the same directory as your executable or in the system library path.

2. **Connection failures**: Check SSH server credentials and network connectivity.

3. **Port already in use**: Ensure the ports you're trying to bind to are available.

4. **Permission denied**: Some ports (< 1024) require administrative privileges.

### Debug Mode

Enable debug mode in your configuration file to get detailed logging:

```ini
[common]
debug = true
```

## Platform-Specific Notes

### Windows
- Requires Visual C++ Redistributable
- May require Windows Defender exclusions for network applications

### macOS
- May require allowing network access through the firewall
- Unsigned binaries may trigger security warnings

### Linux
- Ensure required shared libraries are installed
- May need to adjust firewall rules

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is open source. Please check the license file for details.

## Support

For issues and questions:
1. Check the troubleshooting section
2. Review existing GitHub issues
3. Create a new issue with detailed information about your problem