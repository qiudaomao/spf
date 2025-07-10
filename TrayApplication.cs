using System;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using System.Windows.Forms;

namespace spf
{
    public class TrayApplication
    {
        private readonly AppConfig _config;
        private readonly SshManager _sshManager;
        private readonly Logger _logger;
        private NotifyIcon _trayIcon;
        private ContextMenuStrip _contextMenu;
        private readonly Dictionary<string, ToolStripMenuItem> _serverMenuItems = new();
        private readonly Dictionary<string, ToolStripMenuItem> _forwardMenuItems = new();
        private Icon? _customIcon;

        public TrayApplication(AppConfig config)
        {
            _config = config;
            _sshManager = new SshManager(_config.Servers);
            _logger = Logger.Instance;
            
            InitializeTrayIcon();
            InitializeContextMenu();
        }

        public async Task Initialize()
        {
            _logger.LogInfo("Initializing SSH connections and port forwards...");
            _logger.LogInfo($"Found {_config.Servers.Count} servers and {_config.PortForwards.Count} port forwards in configuration");
            
            // Connect to all servers and start port forwards
            foreach (var forward in _config.PortForwards.Values)
            {
                _logger.LogInfo($"Initializing port forward: {forward.Name}");
                var success = await _sshManager.AddPortForward(forward);
                _logger.LogInfo($"Port forward {forward.Name} initialization result: {success}");
                UpdateForwardMenuItemStatus(forward.Name, success);
            }

            UpdateServerMenuItems();
            UpdateAllForwardMenuItems();
            _logger.LogInfo("Application initialized successfully");
        }

        private void InitializeTrayIcon()
        {
            try
            {
                var iconPath = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "icon.ico");
                if (File.Exists(iconPath))
                {
                    _customIcon = new Icon(iconPath);
                    _logger.LogInfo($"Loaded custom icon from: {iconPath}");
                }
                else
                {
                    _logger.LogWarning($"Icon file not found at: {iconPath}, using default system icon");
                }
            }
            catch (Exception ex)
            {
                _logger.LogError($"Failed to load custom icon: {ex.Message}, using default system icon");
            }

            _trayIcon = new NotifyIcon
            {
                Icon = _customIcon ?? SystemIcons.Application,
                Text = "SSH Port Forwarder",
                Visible = true
            };

            _trayIcon.DoubleClick += (sender, e) => ShowStatus();
        }

        private void InitializeContextMenu()
        {
            _contextMenu = new ContextMenuStrip();
            
            // Servers section
            var serversMenuItem = new ToolStripMenuItem("Servers");
            foreach (var server in _config.Servers)
            {
                var serverItem = new ToolStripMenuItem(server.Key)
                {
                    Tag = server.Key
                };
                serverItem.Click += ServerMenuItem_Click;
                serversMenuItem.DropDownItems.Add(serverItem);
                _serverMenuItems[server.Key] = serverItem;
            }
            _contextMenu.Items.Add(serversMenuItem);

            // Port forwards section
            var forwardsMenuItem = new ToolStripMenuItem("Port Forwards");
            foreach (var forward in _config.PortForwards)
            {
                var forwardItem = new ToolStripMenuItem(forward.Key)
                {
                    Tag = forward.Key
                };
                forwardItem.Click += ForwardMenuItem_Click;
                forwardsMenuItem.DropDownItems.Add(forwardItem);
                _forwardMenuItems[forward.Key] = forwardItem;
            }
            _contextMenu.Items.Add(forwardsMenuItem);

            _contextMenu.Items.Add(new ToolStripSeparator());

            // Status
            var statusMenuItem = new ToolStripMenuItem("Show Status");
            statusMenuItem.Click += (sender, e) => ShowStatus();
            _contextMenu.Items.Add(statusMenuItem);

            _contextMenu.Items.Add(new ToolStripSeparator());

            // Exit
            var exitMenuItem = new ToolStripMenuItem("Exit");
            exitMenuItem.Click += (sender, e) => Exit();
            _contextMenu.Items.Add(exitMenuItem);

            _trayIcon.ContextMenuStrip = _contextMenu;
        }

        private async void ServerMenuItem_Click(object? sender, EventArgs e)
        {
            if (sender is ToolStripMenuItem item && item.Tag is string serverName)
            {
                if (_sshManager.IsConnected(serverName))
                {
                    MessageBox.Show($"Server {serverName} is already connected.", "Server Status", MessageBoxButtons.OK, MessageBoxIcon.Information);
                }
                else
                {
                    var result = MessageBox.Show($"Server {serverName} is disconnected. Try to reconnect?", "Server Status", MessageBoxButtons.YesNo, MessageBoxIcon.Question);
                    if (result == DialogResult.Yes)
                    {
                        await _sshManager.ConnectToServer(serverName);
                        UpdateServerMenuItems();
                    }
                }
            }
        }

        private async void ForwardMenuItem_Click(object? sender, EventArgs e)
        {
            if (sender is ToolStripMenuItem item && item.Tag is string forwardName)
            {
                if (_config.PortForwards.ContainsKey(forwardName))
                {
                    var forward = _config.PortForwards[forwardName];
                    var isRunning = item.Checked;
                    
                    if (isRunning)
                    {
                        _sshManager.RemovePortForward(forward.Server, forwardName);
                        UpdateForwardMenuItemStatus(forwardName, false);
                        _logger.LogInfo($"Port forward stopped: {forwardName}");
                    }
                    else
                    {
                        var success = await _sshManager.AddPortForward(forward);
                        UpdateForwardMenuItemStatus(forwardName, success);
                        if (success)
                        {
                            _logger.LogInfo($"Port forward started: {forwardName}");
                        }
                    }
                }
            }
        }

        private void UpdateServerMenuItems()
        {
            var connectedServers = _sshManager.GetConnectedServers();
            
            foreach (var kvp in _serverMenuItems)
            {
                var isConnected = connectedServers.Contains(kvp.Key);
                kvp.Value.Checked = isConnected;
                kvp.Value.Text = $"{kvp.Key} ({(isConnected ? "Connected" : "Disconnected")})";
            }
        }

        private void UpdateForwardMenuItemStatus(string forwardName, bool isActive)
        {
            if (_forwardMenuItems.ContainsKey(forwardName))
            {
                var item = _forwardMenuItems[forwardName];
                item.Checked = isActive;
                item.Text = $"{forwardName} ({(isActive ? "Active" : "Inactive")})";
            }
        }

        private void UpdateAllForwardMenuItems()
        {
            foreach (var forward in _config.PortForwards.Values)
            {
                var isActive = _sshManager.IsPortForwardActive(forward.Server, forward.Name);
                UpdateForwardMenuItemStatus(forward.Name, isActive);
            }
        }

        private void ShowStatus()
        {
            var connectedServers = _sshManager.GetConnectedServers();
            var activeForwards = _forwardMenuItems.Where(kvp => kvp.Value.Checked).Select(kvp => kvp.Key);
            
            var status = $"SSH Port Forwarder Status\n\n";
            status += $"Connected Servers ({connectedServers.Count}/{_config.Servers.Count}):\n";
            
            foreach (var server in _config.Servers.Keys)
            {
                var isConnected = connectedServers.Contains(server);
                status += $"  • {server}: {(isConnected ? "Connected" : "Disconnected")}\n";
            }
            
            status += $"\nActive Port Forwards ({activeForwards.Count()}/{_config.PortForwards.Count}):\n";
            
            foreach (var forward in _config.PortForwards.Values)
            {
                var isActive = activeForwards.Contains(forward.Name);
                status += $"  • {forward.Name}: {(isActive ? "Active" : "Inactive")}\n";
                status += $"    {forward.Direction} {forward.LocalIP}:{forward.LocalPort} -> {forward.RemoteIP}:{forward.RemotePort}\n";
            }

            MessageBox.Show(status, "SSH Port Forwarder Status", MessageBoxButtons.OK, MessageBoxIcon.Information);
        }

        private void Exit()
        {
            _logger.LogInfo("Application exiting...");
            _sshManager.Dispose();
            _trayIcon.Visible = false;
            _trayIcon.Dispose();
            _customIcon?.Dispose();
            Application.Exit();
        }
    }
}
