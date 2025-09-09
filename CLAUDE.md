# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a PyQt5-based SSH Port Forwarder application that provides a GUI for managing SSH tunnels and port forwarding rules. The application features real-time status monitoring, automatic reconnection, system tray integration, and Windows auto-start support.

## Development Commands

### Running the Application
```bash
python main.py
```

### Installing Dependencies
```bash
pip install -r requirements.txt
```

### Testing
Run the test script to validate functionality:
```bash
python test_tunnel.py
```

### Building Executable (Optional)
```bash
pip install pyinstaller
pyinstaller --windowed --onefile main.py
```

## Architecture Overview

The application follows a modular architecture with clear separation of concerns:

### Core Components
- **main.py**: Application entry point and coordination - initializes all components and manages application lifecycle
- **main_window.py**: Main GUI window using PyQt5 - handles user interactions for server/port forward management
- **database.py**: SQLite database management - stores SSH server configs and port forwarding rules
- **ssh_manager.py**: SSH connection and port forwarding logic using paramiko - handles tunnel creation/management
- **system_tray.py**: System tray integration - provides background operation and quick access menu
- **autostart.py**: Windows auto-start functionality - manages Windows startup registry entries

### Data Flow
1. User configurations stored in SQLite database (`ssh_port_forwarder.db`)
2. SSH Manager creates paramiko connections based on database configs
3. Main Window provides GUI for CRUD operations on servers/port forwards
4. System Tray provides background operation and status indicators
5. Auto-start Manager handles Windows registry for boot startup

### Key Design Patterns
- **Component Injection**: Main components are injected into each other (db_manager, ssh_manager passed to main_window)
- **Event-driven**: Uses PyQt signals/slots for component communication
- **Background Operations**: SSH tunnels run in separate threads managed by ssh_manager
- **Graceful Degradation**: Components check for dependencies and disable features if unavailable

## Important Implementation Details

### Database Schema
The SQLite database contains tables for:
- `servers`: SSH server configurations (host, port, username, auth method)
- `port_forwards`: Port forwarding rules linked to servers
- Configuration uses plain text password storage (security consideration noted in README)

### SSH Connection Management
- Uses paramiko for SSH connections
- Supports password and SSH key authentication (RSA, ECDSA, Ed25519)
- Implements automatic retry logic for failed connections
- Tunnels bind to localhost (127.0.0.1) for security

### System Integration
- Windows auto-start via registry manipulation
- System tray notifications and status icons
- Minimize-to-tray behavior option
- Periodic status updates (5-second timer)

## Security Considerations

- Passwords stored in plain text in local SQLite database
- SSH keys are preferred for better security
- All port forwards bind to localhost only
- Application logs to `ssh_port_forwarder.log` (may contain sensitive info)