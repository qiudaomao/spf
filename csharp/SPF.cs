using System;
using System.Runtime.InteropServices;

namespace SPF
{
    /// <summary>
    /// C# wrapper for the SPF (SSH Port Forwarding) library
    /// </summary>
    public class SPFClient : IDisposable
    {
        private int _instanceId = -1;
        private bool _disposed = false;

        // P/Invoke declarations
        [DllImport("libspf", CallingConvention = CallingConvention.Cdecl)]
        private static extern int SPF_Create([MarshalAs(UnmanagedType.LPStr)] string configPath);

        [DllImport("libspf", CallingConvention = CallingConvention.Cdecl)]
        private static extern int SPF_Start(int instanceID);

        [DllImport("libspf", CallingConvention = CallingConvention.Cdecl)]
        private static extern int SPF_Stop(int instanceID);

        [DllImport("libspf", CallingConvention = CallingConvention.Cdecl)]
        private static extern int SPF_Destroy(int instanceID);

        [DllImport("libspf", CallingConvention = CallingConvention.Cdecl)]
        private static extern int SPF_IsRunning(int instanceID);

        [DllImport("libspf", CallingConvention = CallingConvention.Cdecl)]
        private static extern IntPtr SPF_GetLastError();

        /// <summary>
        /// Initialize a new SPF client with the specified configuration file
        /// </summary>
        /// <param name="configPath">Path to the SPF configuration file</param>
        /// <exception cref="SPFException">Thrown when initialization fails</exception>
        public SPFClient(string configPath)
        {
            if (string.IsNullOrEmpty(configPath))
                throw new ArgumentNullException(nameof(configPath));

            _instanceId = SPF_Create(configPath);
            if (_instanceId < 0)
            {
                string error = GetLastError();
                throw new SPFException($"Failed to create SPF instance: {error}");
            }
        }

        /// <summary>
        /// Start the port forwarding
        /// </summary>
        /// <exception cref="SPFException">Thrown when starting fails</exception>
        public void Start()
        {
            ThrowIfDisposed();
            
            int result = SPF_Start(_instanceId);
            if (result != 0)
            {
                string error = GetLastError();
                throw new SPFException($"Failed to start SPF: {error}");
            }
        }

        /// <summary>
        /// Stop the port forwarding
        /// </summary>
        /// <exception cref="SPFException">Thrown when stopping fails</exception>
        public void Stop()
        {
            ThrowIfDisposed();
            
            int result = SPF_Stop(_instanceId);
            if (result != 0)
            {
                string error = GetLastError();
                throw new SPFException($"Failed to stop SPF: {error}");
            }
        }

        /// <summary>
        /// Check if the port forwarding is currently running
        /// </summary>
        /// <returns>True if running, false otherwise</returns>
        public bool IsRunning
        {
            get
            {
                ThrowIfDisposed();
                
                int result = SPF_IsRunning(_instanceId);
                if (result < 0)
                {
                    string error = GetLastError();
                    throw new SPFException($"Failed to check SPF status: {error}");
                }
                return result == 1;
            }
        }

        /// <summary>
        /// Get the last error message from the native library
        /// </summary>
        /// <returns>Error message or empty string</returns>
        private static string GetLastError()
        {
            IntPtr errorPtr = SPF_GetLastError();
            if (errorPtr == IntPtr.Zero)
                return string.Empty;
            
            string error = Marshal.PtrToStringAnsi(errorPtr);
            Marshal.FreeCoTaskMem(errorPtr); // Free the memory allocated by the C library
            return error ?? string.Empty;
        }

        private void ThrowIfDisposed()
        {
            if (_disposed)
                throw new ObjectDisposedException(nameof(SPFClient));
        }

        /// <summary>
        /// Dispose of the SPF instance
        /// </summary>
        public void Dispose()
        {
            Dispose(true);
            GC.SuppressFinalize(this);
        }

        protected virtual void Dispose(bool disposing)
        {
            if (!_disposed)
            {
                if (_instanceId >= 0)
                {
                    SPF_Destroy(_instanceId);
                    _instanceId = -1;
                }
                _disposed = true;
            }
        }

        ~SPFClient()
        {
            Dispose(false);
        }
    }

    /// <summary>
    /// Exception thrown by SPF operations
    /// </summary>
    public class SPFException : Exception
    {
        public SPFException(string message) : base(message) { }
        public SPFException(string message, Exception innerException) : base(message, innerException) { }
    }
}