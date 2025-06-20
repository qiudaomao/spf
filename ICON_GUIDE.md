# Icon Guide for SSH Port Forwarder

## Creating ICO Files

### Method 1: Online Converters
1. **ConvertICO.com** - Upload PNG/JPG and convert to ICO
2. **ICOConvert.com** - Simple online converter
3. **Favicon.io** - Create icons from text or images

### Method 2: Image Editing Software
- **GIMP** (Free): File → Export As → Select ICO format
- **Photoshop**: File → Save As → Select ICO format
- **Paint.NET** (Free): File → Save As → Select ICO format

### Method 3: Command Line Tools
```bash
# Using ImageMagick (if installed)
convert icon.png -resize 16x16,32x32,48x48 icon.ico

# Using FFmpeg (if installed)
ffmpeg -i icon.png -vf scale=16:16,32:32,48:48 icon.ico
```

## ICO File Requirements

### Recommended Sizes
- **16x16** - System tray icon (required)
- **32x32** - File Explorer icon
- **48x48** - Desktop shortcut icon
- **256x256** - High DPI displays

### File Format
- **Format**: ICO (Windows Icon)
- **Color Depth**: 32-bit (RGBA) recommended
- **Transparency**: Supported (use PNG as source)

## Icon Placement

### For System Tray Icon
- Place `icon.ico` in the same directory as `spf.exe`
- The app will automatically load this icon for the system tray

### For Executable Icon
- Place `icon.ico` in the same directory as your source code
- Use the build scripts to embed the icon into the .exe file
- The icon will appear in File Explorer, taskbar, and shortcuts

## Build Scripts

### Windows Batch (MinGW)
```cmd
build_windows.bat
```

### Windows Batch (Visual Studio)
```cmd
build_windows_vs.bat
```

### PowerShell (Auto-detects compiler)
```powershell
.\build_windows.ps1
```

## Troubleshooting

### Icon Not Showing in System Tray
- Ensure `icon.ico` is in the same directory as `spf.exe`
- Check that the ICO file is valid (try opening it in Windows)
- Verify the file has 16x16 pixel size included

### Executable Icon Not Showing
- Make sure you used a build script that embeds the icon
- Check that `resource.rc` and `icon.ico` are in the source directory
- Verify you have a resource compiler installed (windres or rc.exe)

### Build Errors
- **"windres not found"**: Install MinGW or use Visual Studio build tools
- **"rc.exe not found"**: Install Visual Studio Build Tools
- **"icon.ico not found"**: Place the icon file in the project directory

## Example ICO Creation Workflow

1. **Create/Find Image**: Start with a PNG or JPG image (preferably square)
2. **Resize**: Create versions at 16x16, 32x32, 48x48, and 256x256 pixels
3. **Convert**: Use an online converter or image editor to create ICO file
4. **Test**: Open the ICO file in Windows to verify it displays correctly
5. **Place**: Put `icon.ico` in your project directory
6. **Build**: Use one of the build scripts to create the executable with embedded icon 