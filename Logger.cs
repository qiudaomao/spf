using System;
using System.IO;

namespace spf
{
    public class Logger
    {
        private static Logger? _instance;
        private static readonly object _lock = new object();
        private bool _debugEnabled;
        private readonly string _logFile;

        private Logger()
        {
            _logFile = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "spf.log");
        }

        public static Logger Instance
        {
            get
            {
                if (_instance == null)
                {
                    lock (_lock)
                    {
                        if (_instance == null)
                        {
                            _instance = new Logger();
                        }
                    }
                }
                return _instance;
            }
        }

        public void SetDebugMode(bool enabled)
        {
            _debugEnabled = enabled;
        }

        public void Log(string message, LogLevel level = LogLevel.Info)
        {
            if (!_debugEnabled)
                return;

            var timestamp = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss");
            var logEntry = $"[{timestamp}] [{level}] {message}";
            
            try
            {
                lock (_lock)
                {
                    File.AppendAllText(_logFile, logEntry + Environment.NewLine);
                }
            }
            catch
            {
                // Silently ignore logging errors
            }
        }

        public void LogInfo(string message) => Log(message, LogLevel.Info);
        public void LogError(string message) => Log(message, LogLevel.Error);
        public void LogWarning(string message) => Log(message, LogLevel.Warning);
        public void LogDebug(string message) => Log(message, LogLevel.Debug);
    }

    public enum LogLevel
    {
        Debug,
        Info,
        Warning,
        Error
    }
}