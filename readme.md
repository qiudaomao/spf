# SPF - SSH Port Forwarder

Run spf with a config.ini in same directory.

## Supported Directions

- **local**: Local port forwarding (SSH -L)
- **remote**: Remote port forwarding (SSH -R) 
- **socks5**: SOCKS5 proxy through SSH tunnel (with optional authentication)
- **reverse-socks5**: Reverse SOCKS5 proxy (remote server accesses local network, with optional authentication)

## Configuration

```ini
[serverA]
server=192.168.111.92
user=root
password=abc123

[ssh]
server=serverA
remoteIP=127.0.0.1
remotePort=22
localIP=127.0.0.1
localPort=8922
direction=local

[rdp]
server=serverA
remoteIP=0.0.0.0
remotePort=2289
localIP=127.0.0.1
localPort=3389
direction=remote

[socks5]
server=serverA
localIP=127.0.0.1
localPort=1080
direction=socks5

[socks5-auth]
server=serverA
localIP=127.0.0.1
localPort=1082
direction=socks5
socks5User=proxyuser
socks5Pass=proxypass

[reverse-socks5]
server=serverA
remoteIP=127.0.0.1
remotePort=1081
direction=reverse-socks5

[reverse-socks5-auth]
server=serverA
remoteIP=127.0.0.1
remotePort=1083
direction=reverse-socks5
socks5User=reverseuser
socks5Pass=reversepass
```

## Usage Examples

### Local Port Forwarding
Forwards local port 8922 to remote 127.0.0.1:22 through the SSH server.

### Remote Port Forwarding  
Makes the SSH server listen on 0.0.0.0:2289 and forward connections to local 127.0.0.1:3389.

### SOCKS5 Proxy (No Authentication)
Creates a SOCKS5 proxy server on local port 1080 that routes all traffic through the SSH tunnel. Configure your applications to use `127.0.0.1:1080` as SOCKS5 proxy.

### SOCKS5 Proxy (With Authentication)
Creates a SOCKS5 proxy server on local port 1082 with username/password authentication. Configure your applications to use:
- **Proxy**: `127.0.0.1:1082`
- **Username**: `proxyuser`
- **Password**: `proxypass`

### Reverse SOCKS5 Proxy (No Authentication)
Creates a SOCKS5 proxy server on the remote server (port 1081) that routes traffic back to your local network. Applications on the remote server can use `127.0.0.1:1081` as SOCKS5 proxy to access your local network resources.

### Reverse SOCKS5 Proxy (With Authentication)
Creates an authenticated SOCKS5 proxy server on the remote server (port 1083). Applications on the remote server must authenticate with:
- **Proxy**: `127.0.0.1:1083`
- **Username**: `reverseuser`
- **Password**: `reversepass`

## SOCKS5 Authentication

SOCKS5 authentication is optional and controlled by the presence of `socks5User` and `socks5Pass` fields in the configuration:

- **No Authentication**: Omit `socks5User` and `socks5Pass` fields for open proxy access
- **Username/Password Authentication**: Include both `socks5User` and `socks5Pass` fields to require authentication

The authentication uses the standard SOCKS5 username/password authentication method (RFC 1929).

## Notes

- For SOCKS5 direction, `remoteIP` and `remotePort` are not needed as the target is determined dynamically by the SOCKS5 protocol.
- For reverse-socks5 direction, `localIP` and `localPort` are not needed as connections are made directly to the local network from the remote server.
- Authentication credentials are transmitted securely through the encrypted SSH tunnel.
