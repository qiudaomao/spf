using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using SPF;

namespace SPFConsole
{
    class Program
    {
        private static SPFClient? _spfClient;
        private static ConfigurationData? _configData;
        private static CancellationTokenSource _cancellationTokenSource = new();

        static async Task Main(string[] args)
        {
            Console.WriteLine("SPF C# Console Application");
            Console.WriteLine("===========================");

            // Check if config file path is provided
            string configPath = args.Length > 0 ? args[0] : "config.ini";

            if (!File.Exists(configPath))
            {
                Console.WriteLine($"Config file not found: {configPath}");
                Console.WriteLine("Creating example config file...");
                CreateExampleConfig(configPath);
                Console.WriteLine($"Example config created at: {configPath}");
                Console.WriteLine("Please edit the config file with your SSH server details and run again.");
                return;
            }

            try
            {
                // Parse configuration
                _configData = ParseConfiguration(configPath);

                // Display configuration summary
                DisplayConfigurationSummary();

                // Create SPF instance
                using (_spfClient = new SPFClient(configPath))
                {
                    Console.WriteLine("SPF instance created successfully.");

                    // Start port forwarding
                    Console.WriteLine("Starting port forwarding...");
                    _spfClient.Start();

                    Console.WriteLine($"Port forwarding started. Status: {(_spfClient.IsRunning ? "Running" : "Stopped")}");
                    Console.WriteLine();
                    Console.WriteLine("Commands:");
                    Console.WriteLine("  'status' or 's' - Show status");
                    Console.WriteLine("  'list' or 'l' - List configurations");
                    Console.WriteLine("  'servers' - Show server information");
                    Console.WriteLine("  'stop' - Stop port forwarding");
                    Console.WriteLine("  'start' or 'restart' - (Re)start port forwarding");
                    Console.WriteLine("  'quit' or 'q' - Exit application");
                    Console.WriteLine();

                    // Main command loop
                    await RunCommandLoop();
                }
            }
            catch (SPFException ex)
            {
                Console.WriteLine($"SPF Error: {ex.Message}");
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Unexpected error: {ex.Message}");
            }
        }

        private static async Task RunCommandLoop()
        {
            while (!_cancellationTokenSource.Token.IsCancellationRequested)
            {
                Console.Write("SPF> ");
                var input = Console.ReadLine()?.Trim().ToLower();

                if (string.IsNullOrEmpty(input))
                    continue;

                switch (input)
                {
                    case "quit":
                    case "q":
                    case "exit":
                        Console.WriteLine("Shutting down...");
                        _cancellationTokenSource.Cancel();
                        return;

                    case "status":
                    case "s":
                        ShowStatus();
                        break;

                    case "list":
                    case "l":
                        ListConfigurations();
                        break;

                    case "servers":
                        ShowServers();
                        break;

                    case "stop":
                        StopForwarding();
                        break;

                    case "start":
                    case "restart":
                        RestartForwarding();
                        break;

                    case "help":
                    case "h":
                    case "?":
                        ShowHelp();
                        break;

                    default:
                        // Check if it's a configuration number
                        if (int.TryParse(input, out int configIndex))
                        {
                            ShowConfigurationDetails(configIndex - 1);
                        }
                        else
                        {
                            Console.WriteLine($"Unknown command: {input}. Type 'help' for available commands.");
                        }
                        break;
                }

                Console.WriteLine();
            }
        }

        private static void ShowStatus()
        {
            Console.WriteLine("=== SPF Status ===");
            if (_spfClient != null)
            {
                bool isRunning = _spfClient.IsRunning;
                Console.WriteLine($"Status: {(isRunning ? "Running" : "Stopped")}");
                
                if (_configData != null)
                {
                    Console.WriteLine($"Total configurations: {_configData.Configurations.Count}");
                    Console.WriteLine($"Total servers: {_configData.Servers.Count}");
                }
            }
            else
            {
                Console.WriteLine("Status: Not initialized");
            }
        }

        private static void ListConfigurations()
        {
            Console.WriteLine("=== Configuration List ===");
            
            if (_configData?.Configurations.Any() != true)
            {
                Console.WriteLine("No configurations found.");
                return;
            }

            var groupedConfigs = _configData.Configurations
                .GroupBy(c => c.ServerName)
                .OrderBy(g => g.Key);

            int index = 1;
            foreach (var serverGroup in groupedConfigs)
            {
                Console.WriteLine($"\nServer: {serverGroup.Key}");
                Console.WriteLine(new string('-', serverGroup.Key.Length + 8));

                foreach (var config in serverGroup.OrderBy(c => c.SectionName))
                {
                    string displayName = GetConfigDisplayName(config);
                    Console.WriteLine($"  {index,2}. {displayName}");
                    index++;
                }
            }

            Console.WriteLine($"\nTip: Type a number (1-{index-1}) to see configuration details");
        }

        private static void ShowServers()
        {
            Console.WriteLine("=== Server Information ===");
            
            if (_configData?.Servers.Any() != true)
            {
                Console.WriteLine("No servers configured.");
                return;
            }

            foreach (var server in _configData.Servers.Values.OrderBy(s => s.Name))
            {
                Console.WriteLine($"\nServer: {server.Name}");
                Console.WriteLine($"  Host: {server.Server}:{server.Port}");
                Console.WriteLine($"  User: {server.User}");
                Console.WriteLine($"  Password: {'*'.Repeat(Math.Min(server.Password.Length, 8))}");

                // Count configurations using this server
                var configCount = _configData.Configurations.Count(c => c.ServerName == server.Name);
                Console.WriteLine($"  Configurations: {configCount}");
            }
        }

        private static void ShowConfigurationDetails(int index)
        {
            if (_configData?.Configurations == null || 
                index < 0 || 
                index >= _configData.Configurations.Count)
            {
                Console.WriteLine("Invalid configuration number.");
                return;
            }

            var config = _configData.Configurations[index];
            Console.WriteLine("=== Configuration Details ===");
            Console.WriteLine($"Section: {config.SectionName}");
            Console.WriteLine($"Server: {config.ServerName}");
            Console.WriteLine($"Direction: {config.Direction}");

            switch (config.Direction.ToLower())
            {
                case "remote":
                    Console.WriteLine($"Remote Port Forward: {config.RemoteIP}:{config.RemotePort} → {config.LocalIP}:{config.LocalPort}");
                    break;
                case "local":
                    Console.WriteLine($"Local Port Forward: {config.LocalIP}:{config.LocalPort} ← {config.RemoteIP}:{config.RemotePort}");
                    break;
                case "socks5":
                    Console.WriteLine($"SOCKS5 Proxy: {config.LocalIP}:{config.LocalPort}");
                    if (!string.IsNullOrEmpty(config.Socks5User))
                    {
                        Console.WriteLine($"SOCKS5 Auth: {config.Socks5User}");
                    }
                    break;
                case "reverse-socks5":
                    Console.WriteLine($"Reverse SOCKS5 Proxy: {config.RemoteIP}:{config.RemotePort}");
                    if (!string.IsNullOrEmpty(config.Socks5User))
                    {
                        Console.WriteLine($"SOCKS5 Auth: {config.Socks5User}");
                    }
                    break;
                default:
                    Console.WriteLine("Unknown configuration type");
                    break;
            }
        }

        private static void StopForwarding()
        {
            if (_spfClient == null)
            {
                Console.WriteLine("SPF client not initialized.");
                return;
            }

            try
            {
                _spfClient.Stop();
                Console.WriteLine("Port forwarding stopped.");
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Error stopping port forwarding: {ex.Message}");
            }
        }

        private static void RestartForwarding()
        {
            if (_spfClient == null)
            {
                Console.WriteLine("SPF client not initialized.");
                return;
            }

            try
            {
                Console.WriteLine("Restarting port forwarding...");
                
                if (_spfClient.IsRunning)
                {
                    _spfClient.Stop();
                    Console.WriteLine("Stopped existing forwarding.");
                }

                _spfClient.Start();
                Console.WriteLine($"Port forwarding restarted. Status: {(_spfClient.IsRunning ? "Running" : "Stopped")}");
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Error restarting port forwarding: {ex.Message}");
            }
        }

        private static void ShowHelp()
        {
            Console.WriteLine("=== Available Commands ===");
            Console.WriteLine("  status, s       - Show current status");
            Console.WriteLine("  list, l         - List all configurations");
            Console.WriteLine("  servers         - Show server information");
            Console.WriteLine("  stop            - Stop port forwarding");
            Console.WriteLine("  start, restart  - (Re)start port forwarding");
            Console.WriteLine("  1-N             - Show details for configuration N");
            Console.WriteLine("  help, h, ?      - Show this help");
            Console.WriteLine("  quit, q, exit   - Exit application");
        }

        private static void DisplayConfigurationSummary()
        {
            if (_configData == null)
                return;

            Console.WriteLine("\n=== Configuration Summary ===");
            Console.WriteLine($"Servers: {_configData.Servers.Count}");
            Console.WriteLine($"Configurations: {_configData.Configurations.Count}");

            if (_configData.Configurations.Any())
            {
                var directionCounts = _configData.Configurations
                    .GroupBy(c => c.Direction.ToLower())
                    .ToDictionary(g => g.Key, g => g.Count());

                Console.WriteLine("Configuration types:");
                foreach (var (direction, count) in directionCounts)
                {
                    Console.WriteLine($"  {direction}: {count}");
                }
            }
            Console.WriteLine();
        }

        private static string GetConfigDisplayName(ConfigurationEntry config)
        {
            return config.Direction.ToLower() switch
            {
                "remote" => $"{config.SectionName} {config.RemoteIP}:{config.RemotePort} r → l {config.LocalIP}:{config.LocalPort}",
                "local" => $"{config.SectionName} {config.LocalIP}:{config.LocalPort} l → r {config.RemoteIP}:{config.RemotePort}",
                "socks5" => $"{config.SectionName} {config.LocalIP}:{config.LocalPort} l ← SOCKS5",
                "reverse-socks5" => $"{config.SectionName} {config.RemoteIP}:{config.RemotePort} r → SOCKS5",
                _ => $"{config.SectionName} (Unknown)"
            };
        }

        private static void CreateExampleConfig(string path)
        {
            string exampleConfig = @"[common]
debug = true

# SSH Server Configuration
[server1]
server = your-ssh-server.com
user = your-username
password = your-password
port = 22

# Local Port Forwarding Example
# Forward local port 8080 to remote port 80
[forward-web]
server = server1
direction = local
localIP = 127.0.0.1
localPort = 8080
remoteIP = 127.0.0.1
remotePort = 80

# SOCKS5 Proxy Example
# Create a SOCKS5 proxy on local port 1080
[socks5-proxy]
server = server1
direction = socks5
localIP = 127.0.0.1
localPort = 1080
# Optional SOCKS5 authentication
# socks5User = username
# socks5Pass = password

# Remote Port Forwarding Example
# Forward remote port 9090 to local port 22 (SSH)
[forward-ssh]
server = server1
direction = remote
remoteIP = 0.0.0.0
remotePort = 9090
localIP = 127.0.0.1
localPort = 22

# Reverse SOCKS5 Proxy Example
# Allow remote server to access internet through your connection
[reverse-socks5]
server = server1
direction = reverse-socks5
remoteIP = 127.0.0.1
remotePort = 1081
";

            File.WriteAllText(path, exampleConfig);
        }

        private static ConfigurationData ParseConfiguration(string configPath)
        {
            var data = new ConfigurationData();
            var lines = File.ReadAllLines(configPath);

            string currentSection = "";
            var sectionData = new Dictionary<string, string>();
            var servers = new Dictionary<string, ServerInfo>();
            var configurations = new List<ConfigurationEntry>();

            foreach (var line in lines)
            {
                var trimmedLine = line.Trim();
                if (string.IsNullOrEmpty(trimmedLine) || trimmedLine.StartsWith("#"))
                    continue;

                if (trimmedLine.StartsWith("[") && trimmedLine.EndsWith("]"))
                {
                    // Process previous section
                    if (!string.IsNullOrEmpty(currentSection) && sectionData.Any())
                    {
                        ProcessSection(currentSection, sectionData, servers, configurations);
                    }

                    // Start new section
                    currentSection = trimmedLine[1..^1];
                    sectionData.Clear();
                }
                else if (trimmedLine.Contains("="))
                {
                    var parts = trimmedLine.Split('=', 2);
                    if (parts.Length == 2)
                    {
                        sectionData[parts[0].Trim()] = parts[1].Trim();
                    }
                }
            }

            // Process last section
            if (!string.IsNullOrEmpty(currentSection) && sectionData.Any())
            {
                ProcessSection(currentSection, sectionData, servers, configurations);
            }

            data.Servers = servers;
            data.Configurations = configurations;
            return data;
        }

        private static void ProcessSection(string sectionName, Dictionary<string, string> sectionData,
            Dictionary<string, ServerInfo> servers, List<ConfigurationEntry> configurations)
        {
            if (sectionName == "DEFAULT" || sectionName == "common")
                return;

            if (sectionData.ContainsKey("user") && sectionData.ContainsKey("password"))
            {
                // This is a server configuration
                servers[sectionName] = new ServerInfo
                {
                    Name = sectionName,
                    Server = sectionData.GetValueOrDefault("server", ""),
                    User = sectionData.GetValueOrDefault("user", ""),
                    Password = sectionData.GetValueOrDefault("password", ""),
                    Port = sectionData.GetValueOrDefault("port", "22")
                };
            }
            else if (sectionData.ContainsKey("server") && sectionData.ContainsKey("direction"))
            {
                // This is a forwarding configuration
                configurations.Add(new ConfigurationEntry
                {
                    SectionName = sectionName,
                    ServerName = sectionData.GetValueOrDefault("server", ""),
                    Direction = sectionData.GetValueOrDefault("direction", ""),
                    LocalIP = sectionData.GetValueOrDefault("localIP", ""),
                    LocalPort = sectionData.GetValueOrDefault("localPort", ""),
                    RemoteIP = sectionData.GetValueOrDefault("remoteIP", ""),
                    RemotePort = sectionData.GetValueOrDefault("remotePort", ""),
                    Socks5User = sectionData.GetValueOrDefault("socks5User", ""),
                    Socks5Pass = sectionData.GetValueOrDefault("socks5Pass", "")
                });
            }
        }
    }
}

// Extension method for string repetition
public static class StringExtensions
{
    public static string Repeat(this char character, int count)
    {
        return new string(character, Math.Max(0, count));
    }
}