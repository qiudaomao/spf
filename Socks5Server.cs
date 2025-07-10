using System;
using System.IO;
using System.Net;
using System.Net.Sockets;
using System.Threading;
using System.Threading.Tasks;

namespace spf
{
    public class Socks5Server
    {
        private readonly string _bindAddress;
        private readonly int _bindPort;
        private readonly Logger _logger;
        private TcpListener? _listener;
        private CancellationTokenSource? _cancellationTokenSource;
        private Task? _serverTask;

        public Socks5Server(string bindAddress, int bindPort)
        {
            _bindAddress = bindAddress;
            _bindPort = bindPort;
            _logger = Logger.Instance;
        }

        public bool IsRunning => _listener != null && _serverTask != null && !_serverTask.IsCompleted;

        public async Task<bool> StartAsync()
        {
            try
            {
                _logger.LogInfo($"Starting SOCKS5 server on {_bindAddress}:{_bindPort}");
                
                _listener = new TcpListener(IPAddress.Parse(_bindAddress), _bindPort);
                _listener.Start();
                
                _cancellationTokenSource = new CancellationTokenSource();
                _serverTask = AcceptClientsAsync(_cancellationTokenSource.Token);
                
                _logger.LogInfo($"SOCKS5 server started successfully on {_bindAddress}:{_bindPort}");
                return true;
            }
            catch (Exception ex)
            {
                _logger.LogError($"Failed to start SOCKS5 server: {ex.Message}");
                return false;
            }
        }

        public void Stop()
        {
            try
            {
                _logger.LogInfo($"Stopping SOCKS5 server on {_bindAddress}:{_bindPort}");
                
                _cancellationTokenSource?.Cancel();
                _listener?.Stop();
                _serverTask?.Wait(TimeSpan.FromSeconds(5));
                
                _logger.LogInfo($"SOCKS5 server stopped");
            }
            catch (Exception ex)
            {
                _logger.LogError($"Error stopping SOCKS5 server: {ex.Message}");
            }
        }

        private async Task AcceptClientsAsync(CancellationToken cancellationToken)
        {
            while (!cancellationToken.IsCancellationRequested && _listener != null)
            {
                try
                {
                    var tcpClient = await _listener.AcceptTcpClientAsync();
                    _ = Task.Run(() => HandleClientAsync(tcpClient, cancellationToken), cancellationToken);
                }
                catch (ObjectDisposedException)
                {
                    // Server stopped
                    break;
                }
                catch (Exception ex)
                {
                    _logger.LogError($"Error accepting SOCKS5 client: {ex.Message}");
                }
            }
        }

        private async Task HandleClientAsync(TcpClient client, CancellationToken cancellationToken)
        {
            try
            {
                using (client)
                {
                    var stream = client.GetStream();
                    
                    // SOCKS5 handshake
                    if (!await HandleHandshakeAsync(stream, cancellationToken))
                    {
                        return;
                    }

                    // SOCKS5 connection request
                    var targetEndpoint = await HandleConnectionRequestAsync(stream, cancellationToken);
                    if (targetEndpoint == null)
                    {
                        return;
                    }

                    // Connect to target and relay data
                    await RelayDataAsync(stream, targetEndpoint, cancellationToken);
                }
            }
            catch (Exception ex)
            {
                _logger.LogError($"Error handling SOCKS5 client: {ex.Message}");
            }
        }

        private async Task<bool> HandleHandshakeAsync(NetworkStream stream, CancellationToken cancellationToken)
        {
            try
            {
                // Read version and number of auth methods
                var buffer = new byte[2];
                await stream.ReadAsync(buffer, 0, 2, cancellationToken);
                
                if (buffer[0] != 0x05) // SOCKS version 5
                {
                    return false;
                }

                var numMethods = buffer[1];
                if (numMethods > 0)
                {
                    // Read auth methods
                    var methods = new byte[numMethods];
                    await stream.ReadAsync(methods, 0, numMethods, cancellationToken);
                }

                // Respond with no authentication required
                var response = new byte[] { 0x05, 0x00 };
                await stream.WriteAsync(response, 0, response.Length, cancellationToken);
                
                return true;
            }
            catch
            {
                return false;
            }
        }

        private async Task<IPEndPoint?> HandleConnectionRequestAsync(NetworkStream stream, CancellationToken cancellationToken)
        {
            try
            {
                // Read connection request
                var buffer = new byte[4];
                await stream.ReadAsync(buffer, 0, 4, cancellationToken);
                
                if (buffer[0] != 0x05 || buffer[1] != 0x01) // Version 5, Connect command
                {
                    await SendConnectionResponseAsync(stream, 0x07, cancellationToken); // Command not supported
                    return null;
                }

                var addressType = buffer[3];
                IPAddress? targetAddress = null;
                int targetPort;

                if (addressType == 0x01) // IPv4
                {
                    var addrBytes = new byte[4];
                    await stream.ReadAsync(addrBytes, 0, 4, cancellationToken);
                    targetAddress = new IPAddress(addrBytes);
                }
                else if (addressType == 0x03) // Domain name
                {
                    var domainLength = new byte[1];
                    await stream.ReadAsync(domainLength, 0, 1, cancellationToken);
                    var domainBytes = new byte[domainLength[0]];
                    await stream.ReadAsync(domainBytes, 0, domainLength[0], cancellationToken);
                    var domain = System.Text.Encoding.ASCII.GetString(domainBytes);
                    
                    try
                    {
                        var addresses = await Dns.GetHostAddressesAsync(domain);
                        targetAddress = addresses[0];
                    }
                    catch
                    {
                        await SendConnectionResponseAsync(stream, 0x04, cancellationToken); // Host unreachable
                        return null;
                    }
                }
                else
                {
                    await SendConnectionResponseAsync(stream, 0x08, cancellationToken); // Address type not supported
                    return null;
                }

                // Read port
                var portBytes = new byte[2];
                await stream.ReadAsync(portBytes, 0, 2, cancellationToken);
                targetPort = (portBytes[0] << 8) | portBytes[1];

                await SendConnectionResponseAsync(stream, 0x00, cancellationToken); // Success
                return new IPEndPoint(targetAddress!, targetPort);
            }
            catch
            {
                return null;
            }
        }

        private async Task SendConnectionResponseAsync(NetworkStream stream, byte status, CancellationToken cancellationToken)
        {
            var response = new byte[] { 0x05, status, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00 };
            await stream.WriteAsync(response, 0, response.Length, cancellationToken);
        }

        private async Task RelayDataAsync(NetworkStream clientStream, IPEndPoint targetEndpoint, CancellationToken cancellationToken)
        {
            TcpClient? targetClient = null;
            try
            {
                targetClient = new TcpClient();
                await targetClient.ConnectAsync(targetEndpoint.Address, targetEndpoint.Port);
                var targetStream = targetClient.GetStream();

                // Start relaying data in both directions
                var task1 = RelayStreamAsync(clientStream, targetStream, cancellationToken);
                var task2 = RelayStreamAsync(targetStream, clientStream, cancellationToken);

                await Task.WhenAny(task1, task2);
            }
            catch (Exception ex)
            {
                _logger.LogDebug($"Error in SOCKS5 relay: {ex.Message}");
            }
            finally
            {
                targetClient?.Close();
            }
        }

        private async Task RelayStreamAsync(NetworkStream from, NetworkStream to, CancellationToken cancellationToken)
        {
            try
            {
                var buffer = new byte[4096];
                int bytesRead;
                while ((bytesRead = await from.ReadAsync(buffer, 0, buffer.Length, cancellationToken)) > 0)
                {
                    await to.WriteAsync(buffer, 0, bytesRead, cancellationToken);
                }
            }
            catch
            {
                // Connection closed
            }
        }

        public void Dispose()
        {
            Stop();
            _cancellationTokenSource?.Dispose();
        }
    }
}