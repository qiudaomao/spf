# SSH Port Forwarder

A PyQt-based GUI application for managing SSH port forwarding tunnels with real-time status monitoring, automatic reconnection, and system tray integration.

## Features

- **Server Management**: Add, edit, and delete SSH server configurations
- **Port Forward Management**: Create and manage multiple port forwarding rules
- **Real-time Monitoring**: Live status updates for all active tunnels
- **Auto-retry**: Automatic reconnection when tunnels fail
- **System Tray Integration**: Run in background with quick access menu
- **Auto-start Support**: 
  - Start specific tunnels automatically when application launches
  - Windows auto-start on boot support
- **Flexible Authentication**: Support for password and SSH key authentication
- **SQLite Database**: All configuration stored locally in SQLite database

## Screenshots

### Main Window
The main application window provides a clean interface for managing servers and port forwards:
- Left panel: SSH server management
- Right panel: Port forwarding rules with real-time status

### System Tray
When minimized, the application runs in the system tray with:
- Quick access to start/stop tunnels
- Status indicators (green = active, blue = inactive)
- Context menu for common operations

## Requirements

- Python 3.7+
- PyQt5
- paramiko (SSH client library)
- cryptography
- pywin32 (Windows only, for auto-start functionality)

## Installation

1. Clone or download this repository
2. Install dependencies:
   ```bash
   pip install -r requirements.txt
   ```

## Usage

### Running the Application

```bash
python main.py
```

### Adding SSH Servers

1. Click "Add Server" in the servers panel
2. Fill in the server details:
   - **Name**: Friendly name for the server
   - **Host**: Hostname or IP address
   - **Port**: SSH port (default: 22)
   - **Username**: SSH username
   - **Authentication**: Choose between password or SSH key
3. Test the connection (optional)
4. Click "Save"

### Creating Port Forwards

1. Click "Add Port Forward" in the port forwards panel
2. Configure the tunnel:
   - **Name**: Friendly name for this tunnel
   - **Server**: Select which SSH server to use
   - **Local Port**: Port on your local machine
   - **Remote Host**: Target host on the remote network (e.g., "localhost", "192.168.1.100")
   - **Remote Port**: Target port on the remote host
   - **Enabled**: Whether this tunnel is active
   - **Auto-start**: Start this tunnel automatically when the application launches
3. Click "Save"

### Managing Tunnels

- **Start/Stop**: Use the Start/Stop buttons to manually control tunnels
- **Enable/Disable**: Temporarily disable tunnels without deleting them
- **Status Monitoring**: The Status column shows real-time connection status:
  - 🟢 Connected
  - 🟡 Connecting/Reconnecting
  - 🔴 Error
  - ⚪ Stopped

### System Tray Features

- **Double-click**: Show/hide main window
- **Right-click**: Context menu with:
  - Quick tunnel start/stop
  - Start all auto-start tunnels
  - Stop all tunnels
  - Settings and exit options

### Windows Auto-start

Enable the application to start automatically when Windows boots:

1. Go to Settings menu → "Enable Auto-start"
2. The application will be added to Windows startup programs
3. Toggle again to disable

## Configuration

All configuration is stored in `ssh_port_forwarder.db` SQLite database in the application directory. This includes:

- SSH server configurations
- Port forwarding rules
- Application settings

## Security Notes

- **Password Storage**: Passwords are stored in plain text in the local SQLite database. For better security, use SSH key authentication.
- **SSH Keys**: The application supports RSA, ECDSA, and Ed25519 private keys.
- **Local Binding**: All port forwards bind to localhost (127.0.0.1) for security.

## Troubleshooting

### Common Issues

1. **"Port already in use"**: 
   - Choose a different local port
   - Check if another application is using the port

2. **SSH connection fails**:
   - Verify server credentials
   - Check network connectivity
   - Ensure SSH server is running and accessible

3. **System tray not visible**:
   - Check if system tray is enabled in your OS
   - Look for the application in hidden tray icons

4. **Auto-start not working**:
   - On Windows, check Windows Startup settings
   - Ensure the application has proper permissions

### Log Files

Application logs are written to `ssh_port_forwarder.log` in the application directory. Check this file for detailed error messages and debugging information.

## Architecture

The application is built with a modular architecture:

- `main.py`: Application entry point and coordination
- `main_window.py`: Main GUI window and user interactions
- `database.py`: SQLite database management and data models
- `ssh_manager.py`: SSH connection and port forwarding logic
- `system_tray.py`: System tray integration and notifications
- `autostart.py`: Windows auto-start functionality

## Development

### Adding Features

The codebase is designed to be extensible. Key areas for enhancement:

- **Authentication**: Add support for additional SSH key types or 2FA
- **Cross-platform**: Extend auto-start support to macOS and Linux
- **Import/Export**: Add configuration backup and restore features
- **Logging**: Enhanced logging and log viewer
- **Themes**: Dark mode and custom themes

### Building Executables

To create a standalone executable:

```bash
pip install pyinstaller
pyinstaller --windowed --onefile main.py
```

## License

This project is open source. Feel free to modify and distribute according to your needs.

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues for bugs and feature requests.