using System;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using System.Windows.Forms;
using Hardcodet.Wpf.TaskbarNotification;
using SPF;

class Program
{
    private static SPFClient? _spfClient;
    private static TaskbarIcon? _notifyIcon;
    private static ConfigurationData? _configData;
    private static readonly object _lockObject = new();
    private static CancellationTokenSource _cancellationTokenSource = new();

    [STAThread]
    static async Task Main(string[] args)
    {
        Application.SetHighDpiMode(HighDpiMode.SystemAware);
        Application.EnableVisualStyles();
        Application.SetCompatibleTextRenderingDefault(false);

        // Check if config file path is provided
        string configPath = args.Length > 0 ? args[0] : "config.ini";

        if (!File.Exists(configPath))
        {
            MessageBox.Show($"Config file not found: {configPath}\nCreating example config file...", 
                "SPF - Configuration", MessageBoxButtons.OK, MessageBoxIcon.Information);
            CreateExampleConfig(configPath);
            MessageBox.Show($"Example config created at: {configPath}\nPlease edit the config file with your SSH server details and restart the application.", 
                "SPF - Configuration Created", MessageBoxButtons.OK, MessageBoxIcon.Information);
            return;
        }

        try
        {
            // Parse configuration for menu structure
            _configData = ParseConfiguration(configPath);

            // Create SPF instance
            _spfClient = new SPFClient(configPath);

            // Create system tray icon
            CreateSystemTrayIcon();

            // Start SPF
            await StartSPFAsync();

            // Run message loop
            Application.Run();
        }
        catch (SPFException ex)
        {
            MessageBox.Show($"SPF Error: {ex.Message}", "SPF Error", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }
        catch (Exception ex)
        {
            MessageBox.Show($"Unexpected error: {ex.Message}", "Error", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }
        finally
        {
            Cleanup();
        }
    }

    private static void CreateSystemTrayIcon()
    {
        _notifyIcon = new TaskbarIcon
        {
            Icon = GetApplicationIcon(),
            ToolTipText = "SSH Port Forwarder - Initializing",
            Visibility = System.Windows.Visibility.Visible
        };

        _notifyIcon.ContextMenu = CreateContextMenu();
        _notifyIcon.TrayMouseDoubleClick += (sender, args) => ShowStatusWindow();
    }

    private static Icon GetApplicationIcon()
    {
        string iconPath = "icon.ico";
        if (File.Exists(iconPath))
        {
            return new Icon(iconPath);
        }
        
        // Fallback to system icon if file not found
        return SystemIcons.Application;
    }

    private static ContextMenu CreateContextMenu()
    {
        var contextMenu = new ContextMenu();

        // Status item
        var statusItem = new MenuItem("Status: Starting...")
        {
            Enabled = false
        };
        contextMenu.MenuItems.Add(statusItem);
        contextMenu.MenuItems.Add("-"); // Separator

        // Add server groups and their configurations
        if (_configData != null)
        {
            var serverGroups = GroupConfigurationsByServer(_configData);
            
            foreach (var serverGroup in serverGroups)
            {
                // Add server header
                var serverItem = new MenuItem($"{serverGroup.Key}")
                {
                    Enabled = false
                };
                contextMenu.MenuItems.Add(serverItem);

                // Add configurations under this server
                foreach (var config in serverGroup.Value)
                {
                    var configItem = new MenuItem($"  {GetConfigDisplayName(config)}")
                    {
                        Tag = config
                    };
                    configItem.Click += OnConfigurationMenuClick;
                    contextMenu.MenuItems.Add(configItem);
                }

                contextMenu.MenuItems.Add("-"); // Separator between servers
            }
        }

        // Control items
        var showStatusItem = new MenuItem("Show Status Window");
        showStatusItem.Click += (sender, args) => ShowStatusWindow();
        contextMenu.MenuItems.Add(showStatusItem);

        contextMenu.MenuItems.Add("-"); // Separator

        var quitItem = new MenuItem("Quit");
        quitItem.Click += OnQuitMenuClick;
        contextMenu.MenuItems.Add(quitItem);

        return contextMenu;
    }

    private static Dictionary<string, List<ConfigurationEntry>> GroupConfigurationsByServer(ConfigurationData configData)
    {
        var groups = new Dictionary<string, List<ConfigurationEntry>>();
        
        foreach (var config in configData.Configurations)
        {
            if (!groups.ContainsKey(config.ServerName))
            {
                groups[config.ServerName] = new List<ConfigurationEntry>();
            }
            groups[config.ServerName].Add(config);
        }

        return groups;
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

    private static void OnConfigurationMenuClick(object? sender, EventArgs e)
    {
        if (sender is MenuItem menuItem && menuItem.Tag is ConfigurationEntry config)
        {
            ShowConfigurationDetails(config);
        }
    }

    private static void ShowConfigurationDetails(ConfigurationEntry config)
    {
        string details = $"=== Configuration Details ===\n" +
                        $"Section: {config.SectionName}\n" +
                        $"Server: {config.ServerName}\n" +
                        $"Direction: {config.Direction}\n";

        details += config.Direction.ToLower() switch
        {
            "remote" => $"Remote Port Forward: {config.RemoteIP}:{config.RemotePort} → {config.LocalIP}:{config.LocalPort}",
            "local" => $"Local Port Forward: {config.LocalIP}:{config.LocalPort} ← {config.RemoteIP}:{config.RemotePort}",
            "socks5" => $"SOCKS5 Proxy: {config.LocalIP}:{config.LocalPort}" +
                       (!string.IsNullOrEmpty(config.Socks5User) ? $"\nSOCKS5 Auth: {config.Socks5User}" : ""),
            "reverse-socks5" => $"Reverse SOCKS5 Proxy: {config.RemoteIP}:{config.RemotePort}" +
                               (!string.IsNullOrEmpty(config.Socks5User) ? $"\nSOCKS5 Auth: {config.Socks5User}" : ""),
            _ => "Unknown configuration type"
        };

        MessageBox.Show(details, "Configuration Details", MessageBoxButtons.OK, MessageBoxIcon.Information);
    }

    private static void ShowStatusWindow()
    {
        lock (_lockObject)
        {
            string status = _spfClient?.IsRunning == true ? "Running" : "Stopped";
            string message = $"SSH Port Forwarder Status: {status}\n\n";
            
            if (_configData != null)
            {
                message += "Configurations:\n";
                foreach (var config in _configData.Configurations)
                {
                    message += $"• {config.SectionName} ({config.Direction})\n";
                }
            }

            MessageBox.Show(message, "SPF Status", MessageBoxButtons.OK, MessageBoxIcon.Information);
        }
    }

    private static void OnQuitMenuClick(object? sender, EventArgs e)
    {
        var result = MessageBox.Show("Are you sure you want to quit SSH Port Forwarder?", 
            "Quit SPF", MessageBoxButtons.YesNo, MessageBoxIcon.Question);
        
        if (result == DialogResult.Yes)
        {
            _cancellationTokenSource.Cancel();
            Application.Exit();
        }
    }

    private static async Task StartSPFAsync()
    {
        try
        {
            if (_spfClient != null)
            {
                _spfClient.Start();
                
                if (_notifyIcon != null)
                {
                    _notifyIcon.ToolTipText = "SSH Port Forwarder - Running";
                    
                    // Update status menu item
                    if (_notifyIcon.ContextMenu?.MenuItems.Count > 0)
                    {
                        _notifyIcon.ContextMenu.MenuItems[0].Text = "Status: Running";
                    }
                }

                // Show notification
                _notifyIcon?.ShowBalloonTip("SPF Started", "SSH Port Forwarder is now running", BalloonIcon.Info);
            }
        }
        catch (Exception ex)
        {
            _notifyIcon?.ShowBalloonTip("SPF Error", $"Failed to start: {ex.Message}", BalloonIcon.Error);
            throw;
        }
    }

    private static void Cleanup()
    {
        try
        {
            _cancellationTokenSource.Cancel();
            
            if (_spfClient != null)
            {
                if (_spfClient.IsRunning)
                {
                    _spfClient.Stop();
                }
                _spfClient.Dispose();
            }

            _notifyIcon?.Dispose();
        }
        catch (Exception ex)
        {
            // Log cleanup errors but don't throw
            Console.WriteLine($"Cleanup error: {ex.Message}");
        }
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

// Data classes for configuration parsing
public class ConfigurationData
{
    public Dictionary<string, ServerInfo> Servers { get; set; } = new();
    public List<ConfigurationEntry> Configurations { get; set; } = new();
}

public class ServerInfo
{
    public string Name { get; set; } = "";
    public string Server { get; set; } = "";
    public string User { get; set; } = "";
    public string Password { get; set; } = "";
    public string Port { get; set; } = "22";
}

public class ConfigurationEntry
{
    public string SectionName { get; set; } = "";
    public string ServerName { get; set; } = "";
    public string Direction { get; set; } = "";
    public string LocalIP { get; set; } = "";
    public string LocalPort { get; set; } = "";
    public string RemoteIP { get; set; } = "";
    public string RemotePort { get; set; } = "";
    public string Socks5User { get; set; } = "";
    public string Socks5Pass { get; set; } = "";
}