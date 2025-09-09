#!/usr/bin/env python3
"""
SSH Port Forwarder - A PyQt application for managing SSH port forwarding tunnels

Features:
- Add and manage SSH servers
- Create and manage port forwarding rules
- Real-time connection status monitoring
- Automatic retry on connection failure
- System tray integration
- Auto-start tunnels on application startup
- Windows auto-start on boot support
"""

import sys
import os
import logging
from PyQt5.QtWidgets import QApplication, QMessageBox
from PyQt5.QtCore import QTimer, Qt
from PyQt5.QtGui import QIcon

# Import our modules
from main_window import MainWindow
from system_tray import SystemTrayManager
from autostart import AutoStartManager
from database import DatabaseManager
from ssh_manager import SSHConnectionManager

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('ssh_port_forwarder.log'),
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

class SSHPortForwarderApp(QApplication):
    def __init__(self, sys_argv):
        super().__init__(sys_argv)
        
        # Set application properties
        self.setApplicationName("SSH Port Forwarder")
        self.setApplicationVersion("1.0.0")
        self.setOrganizationName("SSH Tools")
        self.setOrganizationDomain("sshtools.local")
        
        # Initialize components
        self.db_manager = None
        self.ssh_manager = None
        self.main_window = None
        self.system_tray = None
        self.autostart_manager = None
        
        # Setup application
        self.setup_application()
        
    def setup_application(self):
        """Initialize all application components"""
        try:
            # Initialize database
            self.db_manager = DatabaseManager()
            logger.info("Database initialized successfully")
            
            # Initialize SSH connection manager
            self.ssh_manager = SSHConnectionManager()
            logger.info("SSH connection manager initialized")
            
            # Initialize auto-start manager
            self.autostart_manager = AutoStartManager()
            logger.info("Auto-start manager initialized")
            
            # Create main window
            self.main_window = MainWindow()
            self.main_window.db = self.db_manager
            self.main_window.ssh_manager = self.ssh_manager
            self.main_window.autostart_manager = self.autostart_manager
            
            # Update main window's toggle_autostart method
            self.main_window.toggle_autostart = self.toggle_autostart
            
            # Update autostart menu state
            self.main_window.update_autostart_menu()
            
            # Initialize system tray
            self.system_tray = SystemTrayManager(
                self.main_window, 
                self.db_manager, 
                self.ssh_manager
            )
            
            # Connect system tray signals
            self.system_tray.show_main_window.connect(self.show_main_window)
            self.system_tray.quit_application.connect(self.quit_application)
            
            # Connect main window close event to minimize to tray
            self.main_window.closeEvent = self.on_main_window_close
            
            # Setup periodic tray icon updates
            self.tray_update_timer = QTimer()
            self.tray_update_timer.timeout.connect(self.update_tray_status)
            self.tray_update_timer.start(5000)  # Update every 5 seconds
            
            # Show main window initially
            self.main_window.show()
            
            logger.info("Application setup completed successfully")
            
        except Exception as e:
            logger.error(f"Failed to setup application: {e}")
            QMessageBox.critical(None, "Startup Error", 
                               f"Failed to initialize application:\n{str(e)}")
            sys.exit(1)
    
    def show_main_window(self):
        """Show and raise the main window"""
        if self.main_window:
            self.main_window.show()
            self.main_window.raise_()
            self.main_window.activateWindow()
    
    def quit_application(self):
        """Quit the application gracefully"""
        logger.info("Shutting down application...")
        
        # Stop all SSH tunnels
        if self.ssh_manager:
            self.ssh_manager.stop_all_tunnels()
        
        # Close main window
        if self.main_window:
            self.main_window.close()
        
        # Quit application
        self.quit()
    
    def on_main_window_close(self, event):
        """Handle main window close event"""
        if self.system_tray and self.system_tray.minimize_to_tray_action.isChecked():
            # Minimize to tray instead of closing
            event.ignore()
            self.main_window.hide()
            if not hasattr(self, '_tray_message_shown'):
                self.system_tray.show_message(
                    "SSH Port Forwarder",
                    "Application was minimized to tray. Double-click the tray icon to restore.",
                    self.system_tray.tray_icon.Information
                )
                self._tray_message_shown = True
        else:
            # Actually close the application
            self.quit_application()
            event.accept()
    
    def update_tray_status(self):
        """Update system tray icon based on active tunnels"""
        if self.system_tray and self.ssh_manager:
            has_active_tunnels = len(self.ssh_manager.active_tunnels) > 0
            self.system_tray.update_icon_status(has_active_tunnels)
            self.system_tray.update_port_forwards_menu()
    
    def toggle_autostart(self):
        """Toggle Windows auto-start functionality"""
        if not self.autostart_manager:
            QMessageBox.warning(None, "Error", "Auto-start manager not available.")
            return
        
        try:
            if self.autostart_manager.is_auto_start_enabled():
                self.autostart_manager.disable_auto_start()
                QMessageBox.information(None, "Auto-start Disabled", 
                                      "Application will no longer start automatically with Windows.")
                logger.info("Auto-start disabled")
            else:
                self.autostart_manager.enable_auto_start()
                QMessageBox.information(None, "Auto-start Enabled", 
                                      "Application will now start automatically with Windows.")
                logger.info("Auto-start enabled")
        except Exception as e:
            QMessageBox.critical(None, "Auto-start Error", 
                               f"Failed to toggle auto-start:\n{str(e)}")
            logger.error(f"Auto-start toggle failed: {e}")

def main():
    """Main entry point"""
    # Handle high DPI displays
    if hasattr(Qt, 'AA_EnableHighDpiScaling'):
        QApplication.setAttribute(Qt.AA_EnableHighDpiScaling, True)
    if hasattr(Qt, 'AA_UseHighDpiPixmaps'):
        QApplication.setAttribute(Qt.AA_UseHighDpiPixmaps, True)
    
    # Create and run application
    app = SSHPortForwarderApp(sys.argv)
    
    # Set application icon
    app.setWindowIcon(QIcon())
    
    logger.info("Starting SSH Port Forwarder application...")
    
    try:
        sys.exit(app.exec_())
    except KeyboardInterrupt:
        logger.info("Application interrupted by user")
        app.quit_application()
        sys.exit(0)
    except Exception as e:
        logger.error(f"Unhandled application error: {e}")
        sys.exit(1)

if __name__ == '__main__':
    main()