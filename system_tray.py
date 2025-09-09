from PyQt5.QtWidgets import QSystemTrayIcon, QMenu, QAction, QApplication, QMessageBox
from PyQt5.QtCore import pyqtSignal, QObject
from PyQt5.QtGui import QIcon, QPixmap, QPainter, QColor, QFont
from database import DatabaseManager

class SystemTrayManager(QObject):
    show_main_window = pyqtSignal()
    quit_application = pyqtSignal()
    
    def __init__(self, main_window=None, db_manager=None, ssh_manager=None):
        super().__init__()
        self.main_window = main_window
        self.db_manager = db_manager
        self.ssh_manager = ssh_manager
        
        # Check if system tray is available
        if not QSystemTrayIcon.isSystemTrayAvailable():
            QMessageBox.critical(None, "System Tray",
                               "System tray is not available on this system.")
            return
        
        self.create_tray_icon()
        self.create_tray_menu()
        
        # Connect signals
        self.tray_icon.activated.connect(self.on_tray_icon_activated)
        
        # Show tray icon
        self.tray_icon.show()
    
    def create_tray_icon(self):
        """Create the system tray icon"""
        # Create a simple icon programmatically
        icon = self.create_icon()
        self.tray_icon = QSystemTrayIcon(icon)
        self.tray_icon.setToolTip("SSH Port Forwarder")
    
    def create_icon(self, color=QColor(0, 120, 200)):
        """Create a simple application icon"""
        pixmap = QPixmap(32, 32)
        pixmap.fill(QColor(0, 0, 0, 0))  # Transparent background
        
        painter = QPainter(pixmap)
        painter.setRenderHint(QPainter.Antialiasing)
        
        # Draw a simple network/connection icon
        painter.setPen(color)
        painter.setBrush(color)
        
        # Draw computer/server boxes
        painter.drawRect(2, 10, 8, 12)
        painter.drawRect(22, 10, 8, 12)
        
        # Draw connection lines
        painter.drawLine(10, 16, 22, 16)
        painter.drawLine(16, 12, 16, 20)
        
        # Add some connection dots
        painter.drawEllipse(14, 14, 4, 4)
        
        painter.end()
        return QIcon(pixmap)
    
    def create_tray_menu(self):
        """Create the context menu for the system tray"""
        self.tray_menu = QMenu()
        
        # Show main window action
        show_action = QAction("Show SSH Port Forwarder", self)
        show_action.triggered.connect(self.show_main_window.emit)
        self.tray_menu.addAction(show_action)
        
        self.tray_menu.addSeparator()
        
        # Quick actions submenu
        quick_menu = self.tray_menu.addMenu("Quick Actions")
        
        start_all_action = QAction("Start All Auto-Start Tunnels", self)
        start_all_action.triggered.connect(self.start_all_auto_start_tunnels)
        quick_menu.addAction(start_all_action)
        
        stop_all_action = QAction("Stop All Tunnels", self)
        stop_all_action.triggered.connect(self.stop_all_tunnels)
        quick_menu.addAction(stop_all_action)
        
        # Port forwards submenu (will be populated dynamically)
        self.port_forwards_menu = self.tray_menu.addMenu("Port Forwards")
        
        self.tray_menu.addSeparator()
        
        # Settings submenu
        settings_menu = self.tray_menu.addMenu("Settings")
        
        self.minimize_to_tray_action = QAction("Minimize to Tray", self)
        self.minimize_to_tray_action.setCheckable(True)
        self.minimize_to_tray_action.setChecked(True)
        settings_menu.addAction(self.minimize_to_tray_action)
        
        settings_menu.addSeparator()
        
        # Exit action
        exit_action = QAction("Exit", self)
        exit_action.triggered.connect(self.quit_application.emit)
        self.tray_menu.addAction(exit_action)
        
        # Set the menu to the tray icon
        self.tray_icon.setContextMenu(self.tray_menu)
        
        # Update the port forwards menu
        self.update_port_forwards_menu()
    
    def update_port_forwards_menu(self):
        """Update the port forwards submenu with current port forwards"""
        if not (self.db_manager and self.ssh_manager):
            return
        
        # Clear existing actions
        self.port_forwards_menu.clear()
        
        # Get all port forwards
        port_forwards = self.db_manager.get_port_forwards()
        servers = {s.id: s for s in self.db_manager.get_servers()}
        
        if not port_forwards:
            no_pf_action = QAction("No port forwards configured", self)
            no_pf_action.setEnabled(False)
            self.port_forwards_menu.addAction(no_pf_action)
            return
        
        for pf in port_forwards:
            server_name = servers.get(pf.server_id, type('obj', (object,), {'name': 'Unknown'})).name
            
            # Create action with port forward info
            action_text = f"{pf.name} ({server_name}:{pf.local_port})"
            pf_action = QAction(action_text, self)
            
            # Get current status
            is_active = self.ssh_manager.is_tunnel_active(pf.id)
            status = self.ssh_manager.get_tunnel_status(pf.id)
            
            # Update action text with status
            if is_active:
                status_icon = "🟢" if status == "connected" else "🟡" if status == "connecting" else "🔴"
                pf_action.setText(f"{status_icon} {action_text}")
                pf_action.triggered.connect(lambda checked, pf_id=pf.id: self.stop_tunnel(pf_id))
            else:
                pf_action.setText(f"⚪ {action_text}")
                if pf.enabled:
                    pf_action.triggered.connect(lambda checked, s=servers[pf.server_id], p=pf: self.start_tunnel(s, p))
                else:
                    pf_action.setEnabled(False)
            
            self.port_forwards_menu.addAction(pf_action)
    
    def start_tunnel(self, server, port_forward):
        """Start a specific tunnel"""
        if self.ssh_manager:
            self.ssh_manager.start_tunnel(server, port_forward)
            self.update_port_forwards_menu()
            self.show_message("Tunnel Started", f"Started tunnel: {port_forward.name}")
    
    def stop_tunnel(self, port_forward_id):
        """Stop a specific tunnel"""
        if self.ssh_manager:
            self.ssh_manager.stop_tunnel(port_forward_id)
            self.update_port_forwards_menu()
            self.show_message("Tunnel Stopped", f"Stopped tunnel ID: {port_forward_id}")
    
    def start_all_auto_start_tunnels(self):
        """Start all tunnels marked for auto-start"""
        if not (self.db_manager and self.ssh_manager):
            return
        
        auto_start_pfs = self.db_manager.get_auto_start_port_forwards()
        servers = {s.id: s for s in self.db_manager.get_servers()}
        
        started_count = 0
        for pf in auto_start_pfs:
            if pf.server_id in servers and not self.ssh_manager.is_tunnel_active(pf.id):
                self.ssh_manager.start_tunnel(servers[pf.server_id], pf)
                started_count += 1
        
        self.update_port_forwards_menu()
        self.show_message("Tunnels Started", f"Started {started_count} auto-start tunnels")
    
    def stop_all_tunnels(self):
        """Stop all active tunnels"""
        if self.ssh_manager:
            self.ssh_manager.stop_all_tunnels()
            self.update_port_forwards_menu()
            self.show_message("All Tunnels Stopped", "All active tunnels have been stopped")
    
    def show_message(self, title, message, icon=QSystemTrayIcon.Information):
        """Show a system tray notification"""
        if self.tray_icon:
            self.tray_icon.showMessage(title, message, icon, 3000)  # Show for 3 seconds
    
    def on_tray_icon_activated(self, reason):
        """Handle tray icon activation (clicks)"""
        if reason == QSystemTrayIcon.DoubleClick:
            self.show_main_window.emit()
        elif reason == QSystemTrayIcon.Trigger:
            # Single click - update menu
            self.update_port_forwards_menu()
    
    def hide_to_tray(self):
        """Hide the main window to system tray"""
        if self.main_window:
            self.main_window.hide()
            if not self.minimize_to_tray_action.isChecked():
                self.show_message(
                    "SSH Port Forwarder",
                    "Application was minimized to tray",
                    QSystemTrayIcon.Information
                )
    
    def update_icon_status(self, has_active_tunnels=False):
        """Update the tray icon based on tunnel status"""
        if has_active_tunnels:
            # Green icon when tunnels are active
            icon = self.create_icon(QColor(0, 150, 0))
        else:
            # Blue icon when no tunnels are active
            icon = self.create_icon(QColor(0, 120, 200))
        
        self.tray_icon.setIcon(icon)
    
    def set_managers(self, db_manager, ssh_manager):
        """Set the database and SSH managers"""
        self.db_manager = db_manager
        self.ssh_manager = ssh_manager
        self.update_port_forwards_menu()