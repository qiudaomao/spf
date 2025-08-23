#ifndef SPF_H
#define SPF_H

#ifdef __cplusplus
extern "C" {
#endif

// Create a new SPF instance with the given config file path
// Returns instance ID on success, -1 on error
int SPF_Create(const char* configPath);

// Start port forwarding for the given instance
// Returns 0 on success, -1 on error
int SPF_Start(int instanceID);

// Stop port forwarding for the given instance
// Returns 0 on success, -1 on error
int SPF_Stop(int instanceID);

// Destroy an SPF instance and free resources
// Returns 0 on success, -1 on error
int SPF_Destroy(int instanceID);

// Check if an instance is currently running
// Returns 1 if running, 0 if not running, -1 on error
int SPF_IsRunning(int instanceID);

// Get the last error message (caller should free the returned string)
char* SPF_GetLastError();

#ifdef __cplusplus
}
#endif

#endif // SPF_H