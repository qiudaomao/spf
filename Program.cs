using System;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using System.Windows.Forms;

namespace spf
{
    internal static class Program
    {
        [STAThread]
        static async Task Main()
        {
            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);

            try
            {
                var configPath = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "config.ini");
                var config = ConfigParser.ParseConfig(configPath);
                
                Logger.Instance.SetDebugMode(config.Debug);
                Logger.Instance.LogInfo("Application started");

                var app = new TrayApplication(config);
                await app.Initialize();
                
                Application.Run();
            }
            catch (Exception ex)
            {
                MessageBox.Show($"Failed to start application: {ex.Message}", "Error", MessageBoxButtons.OK, MessageBoxIcon.Error);
                Logger.Instance.LogError($"Application failed to start: {ex.Message}");
            }
        }
    }
}