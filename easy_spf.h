#ifndef EASY_SPF_H
#define EASY_SPF_H

#ifdef __cplusplus
extern "C" {
#endif

// Run the SPF application (equivalent to calling main() in main_windows.go)
// This function will block until the user quits from the systray
// Returns 0 on success, -1 on error
// Requires config.ini and icon.ico files in the current directory
int EasySPF_Run();

#ifdef __cplusplus
}
#endif

#endif // EASY_SPF_H