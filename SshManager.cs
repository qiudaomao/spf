using Renci.SshNet;
using Renci.SshNet.Common;
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace spf
{
    public class SshManager
    {
        private readonly Dictionary<string, SshClient> _sshClients = new();
        private readonly Dictionary<string, List<ForwardedPort>> _portForwards = new();
        private readonly Dictionary<string, List<PortForwardConfig>> _portForwardConfigs = new();
        private readonly Dictionary<string, Socks5Server> _socks5Servers = new();
        private readonly Dictionary<string, ServerConfig> _serverConfigs;
        private readonly Logger _logger;
        private readonly System.Threading.Timer _connectionCheckTimer;
        private readonly object _lock = new object();
        public SshManager(Dictionary<string, ServerConfig> serverConfigs)
        {
            _serverConfigs = serverConfigs;
            _logger = Logger.Instance;
            _connectionCheckTimer = new System.Threading.Timer(CheckConnections, null, TimeSpan.FromSeconds(10), TimeSpan.FromSeconds(10));
        }

        public async Task<bool> ConnectToServer(string serverName)
        {
            if (!_serverConfigs.ContainsKey(serverName))
            {
                _logger.LogError($"Server configuration not found: {serverName}");
                return false;
            }

            var config = _serverConfigs[serverName];
            
            try
            {
                lock (_lock)
                {
                    if (_sshClients.ContainsKey(serverName) && _sshClients[serverName].IsConnected)
                    {
                        _logger.LogInfo($"Already connected to server: {serverName}");
                        return true;
                    }
                }

                _logger.LogInfo($"Connecting to server: {serverName} ({config.Server}:{config.Port})");

                SshClient client;
                ConnectionInfo connectionInfo;
                
                if (!string.IsNullOrEmpty(config.PrivateKey))
                {
                    _logger.LogInfo($"Using private key authentication for {serverName}");
                    var keyFile = new PrivateKeyFile(config.PrivateKey);
                    connectionInfo = new ConnectionInfo(config.Server, config.Port, config.User, new PrivateKeyAuthenticationMethod(config.User, keyFile));
                }
                else
                {
                    _logger.LogInfo($"Using password authentication for {serverName}");
                    connectionInfo = new ConnectionInfo(config.Server, config.Port, config.User, new PasswordAuthenticationMethod(config.User, config.Password));
                }

                // Set connection timeouts
                connectionInfo.Timeout = TimeSpan.FromSeconds(30);
                connectionInfo.RetryAttempts = 3;
                connectionInfo.MaxSessions = 10;
                
                _logger.LogInfo($"Created ConnectionInfo with timeout: {connectionInfo.Timeout}, retries: {connectionInfo.RetryAttempts}");
                
                client = new SshClient(connectionInfo);
                
                _logger.LogInfo($"Attempting SSH connection to {config.Server}:{config.Port} as {config.User}");
                
                try
                {
                    _logger.LogInfo($"Starting SSH connection for {serverName}");
                    client.Connect();
                    _logger.LogInfo($"SSH Connect() method completed for {serverName}");
                    
                    if (client.IsConnected)
                    {
                        _logger.LogInfo($"SSH connection established to {serverName}");
                    }
                    else
                    {
                        _logger.LogError($"SSH connection failed - client is not connected to {serverName}");
                        client.Dispose();
                        return false;
                    }
                }
                catch (TimeoutException)
                {
                    _logger.LogError($"SSH connection to {serverName} timed out after 30 seconds");
                    client.Dispose();
                    throw;
                }
                catch (Exception connectEx)
                {
                    _logger.LogError($"SSH connection failed for {serverName}: {connectEx.Message}");
                    _logger.LogError($"Connection exception type: {connectEx.GetType().Name}");
                    if (connectEx.InnerException != null)
                    {
                        _logger.LogError($"Inner exception: {connectEx.InnerException.Message}");
                    }
                    client.Dispose();
                    throw;
                }

                lock (_lock)
                {
                    if (_sshClients.ContainsKey(serverName))
                    {
                        _sshClients[serverName].Dispose();
                    }
                    _sshClients[serverName] = client;
                }

                _logger.LogInfo($"Successfully connected to server: {serverName}");
                return true;
            }
            catch (Exception ex)
            {
                _logger.LogError($"Failed to connect to server {serverName}: {ex.Message}");
                return false;
            }
        }

        public bool IsConnected(string serverName)
        {
            lock (_lock)
            {
                return _sshClients.ContainsKey(serverName) && _sshClients[serverName].IsConnected;
            }
        }

        public async Task<bool> AddPortForward(PortForwardConfig config)
        {
            _logger.LogInfo($"Attempting to add port forward: {config.Name} for server: {config.Server}");
            
            if (!await ConnectToServer(config.Server))
            {
                _logger.LogError($"Failed to connect to server {config.Server} for port forward {config.Name}");
                return false;
            }

            try
            {
                SshClient client;
                ForwardedPort? forwardedPort = null;
                
                // Get SSH client and create port forward object (inside lock)
                lock (_lock)
                {
                    if (!_sshClients.ContainsKey(config.Server) || !_sshClients[config.Server].IsConnected)
                    {
                        _logger.LogError($"SSH client not connected for server: {config.Server}");
                        return false;
                    }

                    client = _sshClients[config.Server];
                }
                
                _logger.LogInfo($"Creating port forward of type: {config.Direction} for {config.Name}");

                // Create port forward object (no lock needed)
                switch (config.Direction.ToLower())
                {
                    case "remote":
                        _logger.LogInfo($"Creating remote port forward: {config.RemotePort} -> {config.LocalIP}:{config.LocalPort}");
                        forwardedPort = new ForwardedPortRemote(
                            (uint)config.RemotePort, 
                            config.LocalIP, 
                            (uint)config.LocalPort);
                        break;
                    case "local":
                        _logger.LogInfo($"Creating local port forward: {config.LocalIP}:{config.LocalPort} -> {config.RemoteIP}:{config.RemotePort}");
                        forwardedPort = new ForwardedPortLocal(
                            config.LocalIP, 
                            (uint)config.LocalPort, 
                            config.RemoteIP, 
                            (uint)config.RemotePort);
                        break;
                    case "socks5":
                    case "dynamic":
                        _logger.LogInfo($"Creating dynamic SOCKS5 proxy on: {config.LocalIP}:{config.LocalPort}");
                        forwardedPort = new ForwardedPortDynamic(
                            config.LocalIP, 
                            (uint)config.LocalPort);
                        break;
                    case "reverse-socks5":
                        _logger.LogInfo($"Creating reverse SOCKS5: will start local SOCKS5 server on {config.LocalIP}:{config.LocalPort} and forward to remote {config.RemotePort}");
                        
                        // Create remote port forward to expose the local SOCKS5 server
                        forwardedPort = new ForwardedPortRemote(
                            (uint)config.RemotePort,
                            config.LocalIP,
                            (uint)config.LocalPort);
                        break;
                    default:
                        _logger.LogError($"Unsupported direction: {config.Direction}");
                        return false;
                }

                if (forwardedPort != null)
                {
                    try
                    {
                        // Special handling for reverse-socks5: start SOCKS5 server first (outside lock)
                        if (config.Direction.ToLower() == "reverse-socks5")
                        {
                            _logger.LogInfo($"Starting local SOCKS5 server for {config.Name} on {config.LocalIP}:{config.LocalPort}");
                            var socks5Server = new Socks5Server(config.LocalIP, config.LocalPort);
                            if (!await socks5Server.StartAsync())
                            {
                                _logger.LogError($"Failed to start local SOCKS5 server for {config.Name}");
                                return false;
                            }
                            
                            // Store the SOCKS5 server reference
                            lock (_lock)
                            {
                                _socks5Servers[config.Name] = socks5Server;
                            }
                            _logger.LogInfo($"SOCKS5 server started successfully for {config.Name}");
                        }

                        _logger.LogInfo($"Adding port forward to SSH client: {config.Name}");
                        client.AddForwardedPort(forwardedPort);
                        
                        _logger.LogInfo($"Starting port forward: {config.Name} - {GetPortForwardDescription(config)}");
                        forwardedPort.Start();
                        
                        _logger.LogInfo($"Port forward started successfully: {config.Name}");
                        _logger.LogInfo($"Port forward is bound: {forwardedPort.IsStarted}");

                        // Store port forward references (inside lock)
                        lock (_lock)
                        {
                            if (!_portForwards.ContainsKey(config.Server))
                            {
                                _portForwards[config.Server] = new List<ForwardedPort>();
                                _portForwardConfigs[config.Server] = new List<PortForwardConfig>();
                            }
                            _portForwards[config.Server].Add(forwardedPort);
                            _portForwardConfigs[config.Server].Add(config);
                        }

                        _logger.LogInfo($"Port forward fully configured: {config.Name} ({config.Direction}) - {GetPortForwardDescription(config)}");
                        return true;
                    }
                    catch (Exception ex)
                    {
                        _logger.LogError($"Failed to start port forward {config.Name}: {ex.Message}");
                        _logger.LogError($"Inner exception: {ex.InnerException?.Message}");
                        _logger.LogError($"Stack trace: {ex.StackTrace}");
                        return false;
                    }
                }
                else
                {
                    _logger.LogError($"Failed to create port forward object for {config.Name}");
                    return false;
                }
            }
            catch (Exception ex)
            {
                _logger.LogError($"Failed to add port forward {config.Name}: {ex.Message}");
            }

            return false;
        }

        public void RemovePortForward(string serverName, string forwardName)
        {
            lock (_lock)
            {
                if (_portForwards.ContainsKey(serverName) && _portForwardConfigs.ContainsKey(serverName))
                {
                    var forwards = _portForwards[serverName];
                    var configs = _portForwardConfigs[serverName];
                    
                    for (int i = forwards.Count - 1; i >= 0; i--)
                    {
                        if (i < configs.Count && configs[i].Name == forwardName)
                        {
                            try
                            {
                                forwards[i].Stop();
                            }
                            catch { }
                            forwards.RemoveAt(i);
                            configs.RemoveAt(i);
                            
                            // Stop associated SOCKS5 server if it exists
                            if (_socks5Servers.ContainsKey(forwardName))
                            {
                                _socks5Servers[forwardName].Stop();
                                _socks5Servers.Remove(forwardName);
                                _logger.LogInfo($"SOCKS5 server stopped for: {forwardName}");
                            }
                            
                            _logger.LogInfo($"Port forward stopped: {forwardName}");
                            break;
                        }
                    }
                }
            }
        }

        private void CheckConnections(object? state)
        {
            lock (_lock)
            {
                var disconnectedServers = new List<string>();
                
                foreach (var kvp in _sshClients)
                {
                    if (!kvp.Value.IsConnected)
                    {
                        disconnectedServers.Add(kvp.Key);
                    }
                }

                foreach (var serverName in disconnectedServers)
                {
                    _logger.LogWarning($"Server {serverName} disconnected, attempting to reconnect...");
                    Task.Run(async () => await ReconnectServer(serverName));
                }
            }
        }

        private async Task ReconnectServer(string serverName)
        {
            // Store port forward configs before clearing
            var configsToRestore = new List<PortForwardConfig>();
            
            lock (_lock)
            {
                // Properly dispose of the disconnected SSH client
                if (_sshClients.ContainsKey(serverName))
                {
                    try
                    {
                        _sshClients[serverName].Dispose();
                    }
                    catch { }
                    _sshClients.Remove(serverName);
                }

                // Remove existing port forwards
                if (_portForwards.ContainsKey(serverName))
                {
                    foreach (var forward in _portForwards[serverName])
                    {
                        try
                        {
                            forward.Stop();
                        }
                        catch { }
                    }
                    _portForwards[serverName].Clear();
                }

                // Store configs for restoration
                if (_portForwardConfigs.ContainsKey(serverName))
                {
                    configsToRestore.AddRange(_portForwardConfigs[serverName]);
                    _portForwardConfigs[serverName].Clear();
                }
            }

            // Reconnect
            if (await ConnectToServer(serverName))
            {
                _logger.LogInfo($"Successfully reconnected to server: {serverName}");
                
                // Restore port forwards
                foreach (var config in configsToRestore)
                {
                    await AddPortForward(config);
                }
            }
            else
            {
                _logger.LogError($"Failed to reconnect to server: {serverName}");
            }
        }

        public List<string> GetConnectedServers()
        {
            lock (_lock)
            {
                var connected = new List<string>();
                foreach (var kvp in _sshClients)
                {
                    if (kvp.Value.IsConnected)
                    {
                        connected.Add(kvp.Key);
                    }
                }
                return connected;
            }
        }

        public bool IsPortForwardActive(string serverName, string forwardName)
        {
            lock (_lock)
            {
                if (_portForwardConfigs.ContainsKey(serverName))
                {
                    var configs = _portForwardConfigs[serverName];
                    return configs.Any(c => c.Name == forwardName);
                }
                return false;
            }
        }

        public List<string> GetActivePortForwards(string serverName)
        {
            lock (_lock)
            {
                if (_portForwardConfigs.ContainsKey(serverName))
                {
                    return _portForwardConfigs[serverName].Select(c => c.Name).ToList();
                }
                return new List<string>();
            }
        }


        private string GetPortForwardDescription(PortForwardConfig config)
        {
            return config.Direction.ToLower() switch
            {
                "remote" => $"Remote listens on {config.RemotePort} -> forwards to {config.LocalIP}:{config.LocalPort}",
                "local" => $"Local {config.LocalIP}:{config.LocalPort} -> forwards to {config.RemoteIP}:{config.RemotePort}",
                "socks5" or "dynamic" => $"SOCKS5 proxy on {config.LocalIP}:{config.LocalPort}",
                "reverse-socks5" => $"Local SOCKS5 server {config.LocalIP}:{config.LocalPort} exposed on remote port {config.RemotePort}",
                _ => $"{config.LocalIP}:{config.LocalPort} <-> {config.RemoteIP}:{config.RemotePort}"
            };
        }

        public void Dispose()
        {
            // Cancel the timer immediately to prevent further checks
            _connectionCheckTimer?.Change(Timeout.Infinite, Timeout.Infinite);
            _connectionCheckTimer?.Dispose();
            
            // Use parallel disposal for faster cleanup
            lock (_lock)
            {
                // Stop all SOCKS5 servers in parallel
                var socks5Tasks = _socks5Servers.Select(kvp => Task.Run(() =>
                {
                    try
                    {
                        kvp.Value.Stop();
                        kvp.Value.Dispose();
                    }
                    catch { }
                })).ToArray();

                // Stop all port forwards in parallel
                var forwardTasks = _portForwards.SelectMany(kvp => kvp.Value.Select(forward => Task.Run(() =>
                {
                    try
                    {
                        forward.Stop();
                    }
                    catch { }
                }))).ToArray();

                // Close all SSH clients in parallel
                var clientTasks = _sshClients.Select(kvp => Task.Run(() =>
                {
                    try
                    {
                        kvp.Value.Dispose();
                    }
                    catch { }
                })).ToArray();

                // Wait for all tasks with a timeout
                Task.WaitAll(socks5Tasks.Concat(forwardTasks).Concat(clientTasks).ToArray(), TimeSpan.FromSeconds(5));

                // Clear collections
                _socks5Servers.Clear();
                _portForwards.Clear();
                _portForwardConfigs.Clear();
                _sshClients.Clear();
            }
        }
    }
}