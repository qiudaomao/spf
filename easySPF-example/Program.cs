using System;
using System.IO;
using EasySPF;

class Program
{
    static void Main(string[] args)
    {
        Console.WriteLine("EasySPF Example Application");
        Console.WriteLine("===========================");
        Console.WriteLine();

        // Check if config file exists
        string configPath = "config.ini";
        if (!File.Exists(configPath))
        {
            Console.WriteLine($"Config file not found: {configPath}");
            Console.WriteLine("Creating example config file...");
            CreateExampleConfig(configPath);
            Console.WriteLine($"Example config created at: {configPath}");
            Console.WriteLine();
            Console.WriteLine("Please edit the config file with your SSH server details and run again.");
            Console.WriteLine("Press any key to exit...");
            Console.ReadKey();
            return;
        }

        try
        {
            Console.WriteLine("Starting SPF with systray support...");
            Console.WriteLine("The application will show in the system tray.");
            Console.WriteLine("Right-click the tray icon to see your configurations.");
            Console.WriteLine("Use the tray menu to quit the application.");
            Console.WriteLine();
            Console.WriteLine("This console window can be closed - SPF will continue running in the tray.");
            Console.WriteLine();

            // This call will block until the user quits from the systray
            EasySPF.EasySPF.Run();
            
            Console.WriteLine("SPF has been stopped.");
        }
        catch (SPFException ex)
        {
            Console.WriteLine($"SPF Error: {ex.Message}");
            Console.WriteLine();
            Console.WriteLine("Please check:");
            Console.WriteLine("- config.ini exists and is properly formatted");
            Console.WriteLine("- SSH server details are correct");
            Console.WriteLine("- Required native library is present");
        }
        catch (Exception ex)
        {
            Console.WriteLine($"Unexpected error: {ex.Message}");
        }

        Console.WriteLine();
        Console.WriteLine("Press any key to exit...");
        Console.ReadKey();
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

# Remote Port Forwarding Example
# Forward remote port 9090 to local port 22 (SSH)
[forward-ssh]
server = server1
direction = remote
remoteIP = 0.0.0.0
remotePort = 9090
localIP = 127.0.0.1
localPort = 22
";

        File.WriteAllText(path, exampleConfig);
    }
}