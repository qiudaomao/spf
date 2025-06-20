# PowerShell script to build SSH Port Forwarder for Windows with embedded icon

Write-Host "Building SSH Port Forwarder for Windows..." -ForegroundColor Green

# Set environment variables for Windows build
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"

# Check if icon.ico exists
if (-not (Test-Path "icon.ico")) {
    Write-Host "ERROR: icon.ico not found!" -ForegroundColor Red
    Write-Host "Please place an icon.ico file in the current directory." -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 1
}

# Try to find resource compiler
$windres = Get-Command "windres" -ErrorAction SilentlyContinue
$rc = Get-Command "rc" -ErrorAction SilentlyContinue

if ($windres) {
    Write-Host "Using MinGW windres..." -ForegroundColor Cyan
    # Compile the resource file using windres
    windres -o rsrc.syso resource.rc
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: Failed to compile resource file!" -ForegroundColor Red
        Read-Host "Press Enter to exit"
        exit 1
    }
} elseif ($rc) {
    Write-Host "Using Visual Studio rc.exe..." -ForegroundColor Cyan
    # Compile the resource file using rc.exe
    rc resource.rc
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: Failed to compile resource file!" -ForegroundColor Red
        Read-Host "Press Enter to exit"
        exit 1
    }
} else {
    Write-Host "WARNING: No resource compiler found!" -ForegroundColor Yellow
    Write-Host "Building without embedded icon..." -ForegroundColor Yellow
    Write-Host "Install MinGW (windres) or Visual Studio Build Tools (rc.exe) to embed icon." -ForegroundColor Yellow
}

# Build the application
Write-Host "Building application..." -ForegroundColor Cyan
go build -ldflags="-s -w -H windowsgui" -o spf.exe .

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Build failed!" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

# Clean up resource files
if (Test-Path "rsrc.syso") { Remove-Item "rsrc.syso" }
if (Test-Path "resource.res") { Remove-Item "resource.res" }

Write-Host "Build complete! spf.exe has been created." -ForegroundColor Green
if ($windres -or $rc) {
    Write-Host "Icon has been embedded into the executable." -ForegroundColor Green
}
Write-Host ""
Write-Host "To run as a system tray application:" -ForegroundColor Cyan
Write-Host "1. Double-click spf.exe" -ForegroundColor White
Write-Host "2. The application will appear in the system tray" -ForegroundColor White
Write-Host "3. Right-click the tray icon for options" -ForegroundColor White
Write-Host ""
Read-Host "Press Enter to exit" 