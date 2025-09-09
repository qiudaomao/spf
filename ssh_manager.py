import paramiko
import threading
import time
import socket
import select
import logging
from typing import Optional, Callable, Dict, Any
from PyQt5.QtCore import QObject, pyqtSignal, QTimer, QThread
from database import Server, PortForward

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class SSHTunnel(QObject):
    status_changed = pyqtSignal(str)  # 'connected', 'disconnected', 'error', 'connecting'
    error_occurred = pyqtSignal(str)
    
    def __init__(self, server: Server, port_forward: PortForward):
        super().__init__()
        self.server = server
        self.port_forward = port_forward
        self.ssh_client = None
        self.tunnel_thread = None
        self.is_running = False
        self.should_stop = False
        self.retry_count = 0
        self.max_retries = 5
        self.retry_delay = 5  # seconds
        self.server_socket = None
        self.remote_forward_request = None  # For remote forwarding cleanup
        
        # Retry timer
        self.retry_timer = QTimer()
        self.retry_timer.timeout.connect(self.retry_connection)
        self.retry_timer.setSingleShot(True)
        
    def start(self):
        """Start the SSH tunnel"""
        if self.is_running:
            return
            
        self.should_stop = False
        self.retry_count = 0
        self.connect()
    
    def stop(self):
        """Stop the SSH tunnel"""
        self.should_stop = True
        self.retry_timer.stop()
        
        # Close server socket first to stop accepting new connections
        if self.server_socket:
            try:
                self.server_socket.close()
            except Exception as e:
                logger.debug(f"Error closing server socket: {e}")
            self.server_socket = None
        
        # Clean up remote forwarding if active
        if self.remote_forward_request and self.ssh_client:
            try:
                transport = self.ssh_client.get_transport()
                if transport:
                    transport.cancel_port_forward(self.port_forward.local_host, self.port_forward.local_port)
            except Exception as e:
                logger.debug(f"Error canceling remote port forward: {e}")
            self.remote_forward_request = None
        
        # Close SSH client
        if self.ssh_client:
            try:
                self.ssh_client.close()
            except Exception as e:
                logger.debug(f"Error closing SSH client: {e}")
            self.ssh_client = None
        
        # Wait for tunnel thread to finish
        if self.tunnel_thread and self.tunnel_thread.is_alive():
            self.tunnel_thread.join(timeout=3)
            if self.tunnel_thread.is_alive():
                logger.warning("Tunnel thread did not stop gracefully")
        
        self.is_running = False
        self.status_changed.emit('disconnected')
    
    def connect(self):
        """Establish SSH connection and start port forwarding"""
        if self.should_stop:
            return
            
        self.status_changed.emit('connecting')
        
        try:
            self.ssh_client = paramiko.SSHClient()
            self.ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
            
            # Prepare connection parameters
            connect_kwargs = {
                'hostname': self.server.host,
                'port': self.server.port,
                'username': self.server.username,
                'timeout': 10
            }
            
            # Use password or private key
            if self.server.private_key_path:
                try:
                    # Try different key types in order of preference
                    key_path = self.server.private_key_path
                    private_key = None
                    
                    # Try RSA key first
                    try:
                        private_key = paramiko.RSAKey.from_private_key_file(key_path)
                        logger.debug(f"Loaded RSA private key from {key_path}")
                    except Exception:
                        # Try Ed25519
                        try:
                            private_key = paramiko.Ed25519Key.from_private_key_file(key_path)
                            logger.debug(f"Loaded Ed25519 private key from {key_path}")
                        except Exception:
                            # Try ECDSA
                            try:
                                private_key = paramiko.ECDSAKey.from_private_key_file(key_path)
                                logger.debug(f"Loaded ECDSA private key from {key_path}")
                            except Exception:
                                # Try DSS
                                try:
                                    private_key = paramiko.DSSKey.from_private_key_file(key_path)
                                    logger.debug(f"Loaded DSS private key from {key_path}")
                                except Exception as e:
                                    logger.error(f"Failed to load private key from {key_path}: {e}")
                                    if self.server.password:
                                        logger.info("Falling back to password authentication")
                                        connect_kwargs['password'] = self.server.password
                                    else:
                                        raise Exception(f"Unable to load private key and no password provided: {e}")
                    
                    if private_key:
                        connect_kwargs['pkey'] = private_key
                        
                except Exception as e:
                    if self.server.password:
                        logger.info("Private key failed, using password authentication")
                        connect_kwargs['password'] = self.server.password
                    else:
                        raise Exception(f"Authentication failed: {e}")
            elif self.server.password:
                connect_kwargs['password'] = self.server.password
            else:
                raise Exception("No authentication method provided (password or private key required)")
            
            # Connect to SSH server
            self.ssh_client.connect(**connect_kwargs)
            
            # Start port forwarding based on direction
            if self.port_forward.direction == 'remote':
                # Remote port forwarding (SSH -R)
                self.tunnel_thread = threading.Thread(target=self._run_remote_tunnel, daemon=True)
            else:
                # Local port forwarding (SSH -L) - default
                self.tunnel_thread = threading.Thread(target=self._run_local_tunnel, daemon=True)
            
            self.tunnel_thread.start()
            
            self.is_running = True
            self.retry_count = 0
            self.status_changed.emit('connected')
            
        except Exception as e:
            logger.error(f"SSH connection failed: {e}")
            self.error_occurred.emit(str(e))
            self.is_running = False
            self.status_changed.emit('error')
            self._schedule_retry()
    
    def _run_local_tunnel(self):
        """Run local port forwarding tunnel (SSH -L)"""
        server_socket = None
        try:
            # Create a server socket
            server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            server_socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            
            # Try to bind to the local port
            try:
                server_socket.bind((self.port_forward.local_host, self.port_forward.local_port))
            except OSError as e:
                if e.errno == 48:  # Address already in use
                    raise Exception(f"Port {self.port_forward.local_port} is already in use")
                else:
                    raise Exception(f"Failed to bind to port {self.port_forward.local_port}: {e}")
            
            server_socket.listen(5)
            server_socket.settimeout(1.0)  # Non-blocking accept
            
            logger.info(f"Port forwarding started: localhost:{self.port_forward.local_port} -> {self.port_forward.remote_host}:{self.port_forward.remote_port}")
            
            # Store the server socket for cleanup
            self.server_socket = server_socket
            
            while not self.should_stop and self.ssh_client and self.ssh_client.get_transport() and self.ssh_client.get_transport().is_active():
                try:
                    # Accept incoming connections
                    client_socket, addr = server_socket.accept()
                    logger.info(f"Accepted connection from {addr}")
                    
                    # Create a new thread to handle this connection
                    handler_thread = threading.Thread(
                        target=self._handle_connection,
                        args=(client_socket,),
                        daemon=True
                    )
                    handler_thread.start()
                    
                except socket.timeout:
                    continue
                except OSError as e:
                    if not self.should_stop:
                        logger.error(f"Error accepting connection: {e}")
                        break
                except Exception as e:
                    if not self.should_stop:
                        logger.error(f"Error accepting connection: {e}")
                        break
            
        except Exception as e:
            logger.error(f"Tunnel error: {e}")
            self.error_occurred.emit(str(e))
            if not self.should_stop:
                self.status_changed.emit('error')
                self._schedule_retry()
        finally:
            # Clean up server socket
            if server_socket:
                try:
                    server_socket.close()
                except:
                    pass
            self.server_socket = None
    
    def _run_remote_tunnel(self):
        """Run remote port forwarding tunnel (SSH -R)"""
        try:
            if not self.ssh_client or not self.ssh_client.get_transport():
                raise Exception("SSH connection not available")
            
            transport = self.ssh_client.get_transport()
            
            # Request remote port forwarding
            logger.info(f"Requesting remote port forward: {self.port_forward.local_host}:{self.port_forward.local_port} -> {self.port_forward.remote_host}:{self.port_forward.remote_port}")
            
            # Request port forwarding on the remote server
            self.remote_forward_request = transport.request_port_forward(
                self.port_forward.local_host, 
                self.port_forward.local_port
            )
            
            if not self.remote_forward_request:
                raise Exception(f"Failed to request remote port forward on {self.port_forward.local_host}:{self.port_forward.local_port}")
            
            logger.info(f"Remote port forwarding established: {self.port_forward.local_host}:{self.port_forward.local_port} -> {self.port_forward.remote_host}:{self.port_forward.remote_port}")
            
            # Handle incoming connections from remote server
            while not self.should_stop and self.ssh_client and self.ssh_client.get_transport() and self.ssh_client.get_transport().is_active():
                try:
                    # Accept forwarded connections from the remote server
                    channel = transport.accept(timeout=1.0)
                    if channel:
                        logger.info(f"Accepted remote forward connection")
                        # Create a new thread to handle this reverse connection
                        handler_thread = threading.Thread(
                            target=self._handle_reverse_connection,
                            args=(channel,),
                            daemon=True
                        )
                        handler_thread.start()
                except socket.timeout:
                    continue
                except Exception as e:
                    if not self.should_stop:
                        logger.error(f"Error handling remote forward connection: {e}")
                        break
            
        except Exception as e:
            logger.error(f"Remote tunnel error: {e}")
            self.error_occurred.emit(str(e))
            if not self.should_stop:
                self.status_changed.emit('error')
                self._schedule_retry()
        finally:
            # Clean up remote forwarding
            if self.remote_forward_request and self.ssh_client:
                try:
                    transport = self.ssh_client.get_transport()
                    if transport:
                        transport.cancel_port_forward(self.port_forward.local_host, self.port_forward.local_port)
                except Exception as e:
                    logger.debug(f"Error canceling remote port forward: {e}")
                self.remote_forward_request = None
    
    def _handle_connection(self, client_socket):
        """Handle individual client connection"""
        channel = None
        try:
            # Verify SSH transport is still active
            if not self.ssh_client or not self.ssh_client.get_transport() or not self.ssh_client.get_transport().is_active():
                logger.error("SSH transport is not active")
                return
            
            # Create SSH channel
            transport = self.ssh_client.get_transport()
            channel = transport.open_channel(
                'direct-tcpip',
                (self.port_forward.remote_host, self.port_forward.remote_port),
                client_socket.getpeername()
            )
            
            if not channel:
                logger.error("Failed to create SSH channel")
                return
            
            logger.info(f"Established tunnel: {client_socket.getpeername()} -> {self.port_forward.remote_host}:{self.port_forward.remote_port}")
            
            # Set socket timeout for non-blocking operation
            client_socket.settimeout(0.1)
            channel.settimeout(0.1)
            
            # Forward data between client and channel
            while not self.should_stop and channel and not channel.closed:
                try:
                    # Use select for cross-platform compatibility
                    ready_read, _, ready_error = select.select([client_socket, channel], [], [client_socket, channel], 1.0)
                    
                    # Check for errors first
                    if ready_error:
                        logger.debug("Socket error detected, closing connection")
                        break
                    
                    # Handle client -> remote
                    if client_socket in ready_read:
                        try:
                            data = client_socket.recv(4096)
                            if not data:
                                logger.debug("Client closed connection")
                                break
                            channel.send(data)
                        except socket.timeout:
                            pass
                        except Exception as e:
                            logger.debug(f"Error reading from client: {e}")
                            break
                    
                    # Handle remote -> client
                    if channel in ready_read:
                        try:
                            data = channel.recv(4096)
                            if not data:
                                logger.debug("Remote closed connection")
                                break
                            client_socket.send(data)
                        except socket.timeout:
                            pass
                        except Exception as e:
                            logger.debug(f"Error reading from remote: {e}")
                            break
                            
                except select.error as e:
                    logger.debug(f"Select error: {e}")
                    break
                except Exception as e:
                    logger.error(f"Unexpected error in connection handler: {e}")
                    break
            
            logger.debug("Connection handler finished")
            
        except paramiko.SSHException as e:
            logger.error(f"SSH channel error: {e}")
        except Exception as e:
            logger.error(f"Connection handling error: {e}")
        finally:
            # Clean up resources
            try:
                if client_socket:
                    client_socket.close()
            except:
                pass
            try:
                if channel:
                    channel.close()
            except:
                pass
    
    def _handle_reverse_connection(self, channel):
        """Handle reverse connection from remote server to local target"""
        local_socket = None
        try:
            # Connect to the local target (remote_host:remote_port in our config)
            local_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            local_socket.settimeout(10)
            local_socket.connect((self.port_forward.remote_host, self.port_forward.remote_port))
            
            logger.info(f"Connected to local target: {self.port_forward.remote_host}:{self.port_forward.remote_port}")
            
            # Set socket timeout for non-blocking operation
            local_socket.settimeout(0.1)
            channel.settimeout(0.1)
            
            # Forward data between channel (from remote) and local socket
            while not self.should_stop and channel and not channel.closed:
                try:
                    # Use select for cross-platform compatibility
                    ready_read, _, ready_error = select.select([local_socket, channel], [], [local_socket, channel], 1.0)
                    
                    # Check for errors first
                    if ready_error:
                        logger.debug("Socket error detected in reverse connection, closing")
                        break
                    
                    # Handle remote -> local
                    if channel in ready_read:
                        try:
                            data = channel.recv(4096)
                            if not data:
                                logger.debug("Remote closed reverse connection")
                                break
                            local_socket.send(data)
                        except socket.timeout:
                            pass
                        except Exception as e:
                            logger.debug(f"Error reading from remote in reverse connection: {e}")
                            break
                    
                    # Handle local -> remote
                    if local_socket in ready_read:
                        try:
                            data = local_socket.recv(4096)
                            if not data:
                                logger.debug("Local target closed reverse connection")
                                break
                            channel.send(data)
                        except socket.timeout:
                            pass
                        except Exception as e:
                            logger.debug(f"Error reading from local in reverse connection: {e}")
                            break
                            
                except select.error as e:
                    logger.debug(f"Select error in reverse connection: {e}")
                    break
                except Exception as e:
                    logger.error(f"Unexpected error in reverse connection handler: {e}")
                    break
            
            logger.debug("Reverse connection handler finished")
            
        except Exception as e:
            logger.error(f"Reverse connection handling error: {e}")
        finally:
            # Clean up resources
            try:
                if local_socket:
                    local_socket.close()
            except:
                pass
            try:
                if channel:
                    channel.close()
            except:
                pass
    
    def _schedule_retry(self):
        """Schedule a retry connection attempt"""
        if self.should_stop or self.retry_count >= self.max_retries:
            if self.retry_count >= self.max_retries:
                self.error_occurred.emit(f"Max retries ({self.max_retries}) reached")
            return
        
        self.retry_count += 1
        retry_delay = min(self.retry_delay * self.retry_count, 60)  # Max 60 seconds
        logger.info(f"Scheduling retry {self.retry_count}/{self.max_retries} in {retry_delay} seconds")
        self.retry_timer.start(retry_delay * 1000)
    
    def retry_connection(self):
        """Retry the connection"""
        if not self.should_stop:
            logger.info(f"Retrying connection (attempt {self.retry_count}/{self.max_retries})")
            self.connect()

class SSHConnectionManager(QObject):
    tunnel_status_changed = pyqtSignal(int, str)  # port_forward_id, status
    tunnel_error = pyqtSignal(int, str)  # port_forward_id, error_message
    
    def __init__(self):
        super().__init__()
        self.active_tunnels: Dict[int, SSHTunnel] = {}
    
    def start_tunnel(self, server: Server, port_forward: PortForward):
        """Start a port forwarding tunnel"""
        if port_forward.id in self.active_tunnels:
            self.stop_tunnel(port_forward.id)
        
        tunnel = SSHTunnel(server, port_forward)
        tunnel.status_changed.connect(
            lambda status, pf_id=port_forward.id: self.tunnel_status_changed.emit(pf_id, status)
        )
        tunnel.error_occurred.connect(
            lambda error, pf_id=port_forward.id: self.tunnel_error.emit(pf_id, error)
        )
        
        self.active_tunnels[port_forward.id] = tunnel
        tunnel.start()
    
    def stop_tunnel(self, port_forward_id: int):
        """Stop a port forwarding tunnel"""
        if port_forward_id in self.active_tunnels:
            tunnel = self.active_tunnels[port_forward_id]
            tunnel.stop()
            del self.active_tunnels[port_forward_id]
    
    def stop_all_tunnels(self):
        """Stop all active tunnels"""
        for tunnel in list(self.active_tunnels.values()):
            tunnel.stop()
        self.active_tunnels.clear()
    
    def get_tunnel_status(self, port_forward_id: int) -> str:
        """Get the current status of a tunnel"""
        if port_forward_id in self.active_tunnels:
            tunnel = self.active_tunnels[port_forward_id]
            if tunnel.is_running:
                return 'connected'
            else:
                return 'disconnected'
        return 'stopped'
    
    def is_tunnel_active(self, port_forward_id: int) -> bool:
        """Check if a tunnel is active"""
        return port_forward_id in self.active_tunnels