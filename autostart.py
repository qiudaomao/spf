import sys
import os
import platform
import winreg
from pathlib import Path

class AutoStartManager:
    def __init__(self, app_name="SSH Port Forwarder"):
        self.app_name = app_name
        self.is_windows = platform.system() == "Windows"
    
    def get_executable_path(self):
        """Get the path to the current executable or script"""
        if getattr(sys, 'frozen', False):
            # Running as compiled executable
            return sys.executable
        else:
            # Running as Python script
            python_exe = sys.executable
            script_path = os.path.abspath(sys.argv[0])
            return f'"{python_exe}" "{script_path}"'
    
    def is_auto_start_enabled(self):
        """Check if auto-start is currently enabled"""
        if not self.is_windows:
            return False
        
        try:
            key = winreg.OpenKey(
                winreg.HKEY_CURRENT_USER,
                r"SOFTWARE\Microsoft\Windows\CurrentVersion\Run",
                0,
                winreg.KEY_READ
            )
            try:
                value, _ = winreg.QueryValueEx(key, self.app_name)
                winreg.CloseKey(key)
                return True
            except FileNotFoundError:
                winreg.CloseKey(key)
                return False
        except Exception:
            return False
    
    def enable_auto_start(self):
        """Enable auto-start on Windows boot"""
        if not self.is_windows:
            raise RuntimeError("Auto-start is only supported on Windows")
        
        try:
            executable_path = self.get_executable_path()
            
            key = winreg.OpenKey(
                winreg.HKEY_CURRENT_USER,
                r"SOFTWARE\Microsoft\Windows\CurrentVersion\Run",
                0,
                winreg.KEY_SET_VALUE
            )
            
            winreg.SetValueEx(
                key,
                self.app_name,
                0,
                winreg.REG_SZ,
                executable_path
            )
            
            winreg.CloseKey(key)
            return True
            
        except Exception as e:
            raise RuntimeError(f"Failed to enable auto-start: {str(e)}")
    
    def disable_auto_start(self):
        """Disable auto-start on Windows boot"""
        if not self.is_windows:
            raise RuntimeError("Auto-start is only supported on Windows")
        
        try:
            key = winreg.OpenKey(
                winreg.HKEY_CURRENT_USER,
                r"SOFTWARE\Microsoft\Windows\CurrentVersion\Run",
                0,
                winreg.KEY_SET_VALUE
            )
            
            try:
                winreg.DeleteValue(key, self.app_name)
                winreg.CloseKey(key)
                return True
            except FileNotFoundError:
                winreg.CloseKey(key)
                return True  # Already disabled
                
        except Exception as e:
            raise RuntimeError(f"Failed to disable auto-start: {str(e)}")
    
    def toggle_auto_start(self):
        """Toggle auto-start on/off"""
        if self.is_auto_start_enabled():
            self.disable_auto_start()
            return False
        else:
            self.enable_auto_start()
            return True