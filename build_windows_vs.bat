@echo off
echo Building SSH Port Forwarder for Windows (Visual Studio)...

REM Set environment variables for Windows build
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=1

REM Check if icon.ico exists
if not exist "icon.ico" (
    echo ERROR: icon.ico not found!
    echo Please place an icon.ico file in the current directory.
    pause
    exit /b 1
)

REM Compile the resource file using Visual Studio rc.exe
echo Compiling resource file...
rc resource.rc
if errorlevel 1 (
    echo ERROR: Failed to compile resource file!
    echo Make sure you have Visual Studio Build Tools installed and rc.exe is in PATH.
    pause
    exit /b 1
)

REM Build the application with embedded resources
echo Building application...
go build -ldflags="-s -w -H windowsgui" -o spf.exe .

REM Clean up resource files
del resource.res

echo Build complete! spf.exe has been created with embedded icon.
echo.
echo To run as a system tray application:
echo 1. Double-click spf.exe
echo 2. The application will appear in the system tray
echo 3. Right-click the tray icon for options
echo.
pause 