using System;
using System.Runtime.InteropServices;

namespace EasySPF
{
    /// <summary>
    /// Minimal C# wrapper for SPF that simply calls the main function from main_windows.go
    /// This is the simplest possible integration - just runs the Go systray application
    /// </summary>
    public static class EasySPF
    {
        // P/Invoke declaration for the single exported function
        [DllImport("libeasyspf", CallingConvention = CallingConvention.Cdecl)]
        private static extern int EasySPF_Run();

        /// <summary>
        /// Run the SPF application with systray support
        /// This function will block until the user quits from the systray menu
        /// 
        /// Prerequisites:
        /// - config.ini file must exist in the current directory
        /// - icon.ico file should exist in the current directory (optional)
        /// - libeasyspf.dylib/libeasyspf.so/easyspf.dll must be in the application directory
        /// </summary>
        /// <exception cref="SPFException">Thrown when the SPF application fails to start</exception>
        public static void Run()
        {
            int result = EasySPF_Run();
            if (result != 0)
            {
                throw new SPFException("Failed to run SPF application. Check that config.ini exists and is valid.");
            }
        }

        /// <summary>
        /// Run the SPF application with systray support (async version)
        /// This allows the calling thread to continue while SPF runs in the background
        /// 
        /// Note: The SPF application will still show its own systray and handle its own lifecycle
        /// </summary>
        /// <returns>Task that completes when SPF exits</returns>
        public static System.Threading.Tasks.Task RunAsync()
        {
            return System.Threading.Tasks.Task.Run(() => Run());
        }
    }

    /// <summary>
    /// Exception thrown by EasySPF operations
    /// </summary>
    public class SPFException : Exception
    {
        public SPFException(string message) : base(message) { }
        public SPFException(string message, Exception innerException) : base(message, innerException) { }
    }
}