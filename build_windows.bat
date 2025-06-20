@echo off
echo Building SSH Port Forwarder for Windows...

REM Set environment variables for Windows build
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=1

REM Build the application
go build -ldflags="-s -w -H windowsgui" -o spf.exe .

echo Build complete! spf.exe has been created.
echo.
echo To run as a system tray application:
echo 1. Double-click spf.exe
echo 2. The application will appear in the system tray
echo 3. Right-click the tray icon for options
echo.
pause 