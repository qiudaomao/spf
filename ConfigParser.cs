using Microsoft.Extensions.Configuration;
using System.Collections.Generic;
using System.IO;

namespace spf
{
    public class ServerConfig
    {
        public string Server { get; set; } = "";
        public string User { get; set; } = "";
        public string Password { get; set; } = "";
        public string PrivateKey { get; set; } = "";
        public int Port { get; set; } = 22;
    }

    public class PortForwardConfig
    {
        public string Name { get; set; } = "";
        public string Server { get; set; } = "";
        public string RemoteIP { get; set; } = "";
        public int RemotePort { get; set; }
        public string LocalIP { get; set; } = "";
        public int LocalPort { get; set; }
        public string Direction { get; set; } = "";
    }

    public class AppConfig
    {
        public bool Debug { get; set; }
        public Dictionary<string, ServerConfig> Servers { get; set; } = new();
        public Dictionary<string, PortForwardConfig> PortForwards { get; set; } = new();
    }

    public class ConfigParser
    {
        public static AppConfig ParseConfig(string configPath)
        {
            var config = new AppConfig();
            
            if (!File.Exists(configPath))
            {
                throw new FileNotFoundException($"Config file not found: {configPath}");
            }

            var configuration = new ConfigurationBuilder()
                .AddIniFile(configPath)
                .Build();

            // Parse common section
            var commonSection = configuration.GetSection("common");
            config.Debug = commonSection.GetValue<bool>("debug", false);

            // Parse server configurations
            var serverSections = new List<string> { "iwrt", "gv" };
            foreach (var serverName in serverSections)
            {
                var section = configuration.GetSection(serverName);
                if (section.Exists())
                {
                    var serverConfig = new ServerConfig
                    {
                        Server = section.GetValue<string>("server", ""),
                        User = section.GetValue<string>("user", ""),
                        Password = section.GetValue<string>("password", ""),
                        PrivateKey = section.GetValue<string>("privatekey", ""),
                        Port = section.GetValue<int>("port", 22)
                    };
                    config.Servers[serverName] = serverConfig;
                }
            }

            // Parse port forward configurations
            var portForwardSections = new List<string> { "ssh", "rdp", "adb", "socks5", "synergy-24800", "synergy-24802" };
            foreach (var forwardName in portForwardSections)
            {
                var section = configuration.GetSection(forwardName);
                if (section.Exists())
                {
                    var forwardConfig = new PortForwardConfig
                    {
                        Name = forwardName,
                        Server = section.GetValue<string>("server", ""),
                        RemoteIP = section.GetValue<string>("remoteIP", ""),
                        RemotePort = section.GetValue<int>("remotePort", 0),
                        LocalIP = section.GetValue<string>("localIP", ""),
                        LocalPort = section.GetValue<int>("localPort", 0),
                        Direction = section.GetValue<string>("direction", "")
                    };
                    
                    // Set defaults for localIP and localPort if not provided
                    var originalLocalIP = forwardConfig.LocalIP;
                    var originalLocalPort = forwardConfig.LocalPort;
                    
                    if (string.IsNullOrEmpty(forwardConfig.LocalIP))
                    {
                        forwardConfig.LocalIP = "127.0.0.1";
                    }
                    
                    if (forwardConfig.LocalPort == 0)
                    {
                        forwardConfig.LocalPort = GetRandomAvailablePort();
                    }

                    // Log applied defaults
                    if (string.IsNullOrEmpty(originalLocalIP) || originalLocalPort == 0)
                    {
                        Console.WriteLine($"Applied defaults to {forwardConfig.Name}: LocalIP={forwardConfig.LocalIP}, LocalPort={forwardConfig.LocalPort}");
                    }
                    
                    config.PortForwards[forwardName] = forwardConfig;
                }
            }

            // Auto-discover sections
            var allSections = configuration.GetChildren();
            foreach (var section in allSections)
            {
                var sectionName = section.Key;
                
                // Skip common section
                if (sectionName == "common")
                    continue;

                // Check if it's a server config (has server, user fields)
                if (section.GetValue<string>("server") != null && 
                    section.GetValue<string>("user") != null && 
                    !config.Servers.ContainsKey(sectionName))
                {
                    var serverConfig = new ServerConfig
                    {
                        Server = section.GetValue<string>("server", ""),
                        User = section.GetValue<string>("user", ""),
                        Password = section.GetValue<string>("password", ""),
                        PrivateKey = section.GetValue<string>("privatekey", ""),
                        Port = section.GetValue<int>("port", 22)
                    };
                    config.Servers[sectionName] = serverConfig;
                }
                // Check if it's a port forward config (has remotePort, direction fields)
                else if (section.GetValue<string>("direction") != null &&
                         !config.PortForwards.ContainsKey(sectionName))
                {
                    var forwardConfig = new PortForwardConfig
                    {
                        Name = sectionName,
                        Server = section.GetValue<string>("server", ""),
                        RemoteIP = section.GetValue<string>("remoteIP", ""),
                        RemotePort = section.GetValue<int>("remotePort", 0),
                        LocalIP = section.GetValue<string>("localIP", ""),
                        LocalPort = section.GetValue<int>("localPort", 0),
                        Direction = section.GetValue<string>("direction", "")
                    };
                    
                    // Set defaults for localIP and localPort if not provided
                    var originalLocalIP = forwardConfig.LocalIP;
                    var originalLocalPort = forwardConfig.LocalPort;
                    
                    if (string.IsNullOrEmpty(forwardConfig.LocalIP))
                    {
                        forwardConfig.LocalIP = "127.0.0.1";
                    }
                    
                    if (forwardConfig.LocalPort == 0)
                    {
                        forwardConfig.LocalPort = GetRandomAvailablePort();
                    }

                    // Log applied defaults
                    if (string.IsNullOrEmpty(originalLocalIP) || originalLocalPort == 0)
                    {
                        Console.WriteLine($"Applied defaults to {forwardConfig.Name}: LocalIP={forwardConfig.LocalIP}, LocalPort={forwardConfig.LocalPort}");
                    }
                    
                    config.PortForwards[sectionName] = forwardConfig;
                }
            }

            return config;
        }

        private static int GetRandomAvailablePort()
        {
            using var socket = new System.Net.Sockets.Socket(System.Net.Sockets.AddressFamily.InterNetwork, System.Net.Sockets.SocketType.Stream, System.Net.Sockets.ProtocolType.Tcp);
            socket.Bind(new System.Net.IPEndPoint(System.Net.IPAddress.Loopback, 0));
            var port = ((System.Net.IPEndPoint)socket.LocalEndPoint!).Port;
            return port;
        }

    }
}