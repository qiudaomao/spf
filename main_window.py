import sys
import os
from PyQt5.QtWidgets import (QApplication, QMainWindow, QWidget, QVBoxLayout, 
                             QHBoxLayout, QTabWidget, QTableWidget, QTableWidgetItem,
                             QPushButton, QLabel, QMenuBar, QMenu, QAction, 
                             QSystemTrayIcon, QStyle, QHeaderView, QMessageBox,
                             QDialog, QFormLayout, QLineEdit, QSpinBox, QCheckBox,
                             QComboBox, QFileDialog, QGroupBox, QSplitter,
                             QTextEdit, QProgressBar, QStatusBar)
from PyQt5.QtCore import Qt, QTimer, pyqtSignal, QThread
from PyQt5.QtGui import QIcon, QPixmap, QPainter, QColor, QFont
from database import DatabaseManager, Server, PortForward
from ssh_manager import SSHConnectionManager
from autostart import AutoStartManager

class ServerDialog(QDialog):
    def __init__(self, parent=None, server=None):
        super().__init__(parent)
        self.server = server
        self.is_edit = server is not None
        self.setWindowTitle("Edit Server" if self.is_edit else "Add Server")
        self.setModal(True)
        self.setFixedSize(400, 300)
        self.setup_ui()
        
        if self.is_edit:
            self.load_server_data()
    
    def setup_ui(self):
        layout = QVBoxLayout(self)
        
        # Form layout
        form_layout = QFormLayout()
        
        self.name_edit = QLineEdit()
        self.name_edit.setPlaceholderText("Server name")
        form_layout.addRow("Name:", self.name_edit)
        
        self.host_edit = QLineEdit()
        self.host_edit.setPlaceholderText("hostname or IP")
        form_layout.addRow("Host:", self.host_edit)
        
        self.port_spin = QSpinBox()
        self.port_spin.setRange(1, 65535)
        self.port_spin.setValue(22)
        form_layout.addRow("Port:", self.port_spin)
        
        self.username_edit = QLineEdit()
        self.username_edit.setPlaceholderText("SSH username")
        form_layout.addRow("Username:", self.username_edit)
        
        # Authentication group
        auth_group = QGroupBox("Authentication")
        auth_layout = QVBoxLayout(auth_group)
        
        self.password_edit = QLineEdit()
        self.password_edit.setEchoMode(QLineEdit.Password)
        self.password_edit.setPlaceholderText("Password (optional)")
        auth_layout.addWidget(QLabel("Password:"))
        auth_layout.addWidget(self.password_edit)
        
        key_layout = QHBoxLayout()
        self.key_path_edit = QLineEdit()
        self.key_path_edit.setPlaceholderText("Path to private key (optional)")
        self.browse_key_btn = QPushButton("Browse")
        self.browse_key_btn.clicked.connect(self.browse_private_key)
        key_layout.addWidget(self.key_path_edit)
        key_layout.addWidget(self.browse_key_btn)
        auth_layout.addWidget(QLabel("Private Key:"))
        auth_layout.addLayout(key_layout)
        
        layout.addLayout(form_layout)
        layout.addWidget(auth_group)
        
        # Buttons
        button_layout = QHBoxLayout()
        self.test_btn = QPushButton("Test Connection")
        self.test_btn.clicked.connect(self.test_connection)
        self.save_btn = QPushButton("Save")
        self.save_btn.clicked.connect(self.accept)
        self.cancel_btn = QPushButton("Cancel")
        self.cancel_btn.clicked.connect(self.reject)
        
        button_layout.addWidget(self.test_btn)
        button_layout.addStretch()
        button_layout.addWidget(self.save_btn)
        button_layout.addWidget(self.cancel_btn)
        layout.addLayout(button_layout)
    
    def browse_private_key(self):
        file_path, _ = QFileDialog.getOpenFileName(
            self, "Select Private Key File", "", "All Files (*)")
        if file_path:
            self.key_path_edit.setText(file_path)
    
    def load_server_data(self):
        self.name_edit.setText(self.server.name)
        self.host_edit.setText(self.server.host)
        self.port_spin.setValue(self.server.port)
        self.username_edit.setText(self.server.username)
        if self.server.password:
            self.password_edit.setText(self.server.password)
        if self.server.private_key_path:
            self.key_path_edit.setText(self.server.private_key_path)
    
    def get_server_data(self):
        return Server(
            id=self.server.id if self.is_edit else None,
            name=self.name_edit.text().strip(),
            host=self.host_edit.text().strip(),
            port=self.port_spin.value(),
            username=self.username_edit.text().strip(),
            password=self.password_edit.text() if self.password_edit.text() else None,
            private_key_path=self.key_path_edit.text().strip() if self.key_path_edit.text().strip() else None
        )
    
    def test_connection(self):
        # Simple validation
        if not all([self.name_edit.text().strip(), self.host_edit.text().strip(), 
                   self.username_edit.text().strip()]):
            QMessageBox.warning(self, "Validation Error", "Please fill in all required fields.")
            return
        
        # Disable the test button during testing
        self.test_btn.setEnabled(False)
        self.test_btn.setText("Testing...")
        QApplication.processEvents()
        
        # Test the SSH connection
        try:
            import paramiko
            import socket
            
            ssh_client = paramiko.SSHClient()
            ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
            
            # Prepare connection parameters
            connect_kwargs = {
                'hostname': self.host_edit.text().strip(),
                'port': self.port_spin.value(),
                'username': self.username_edit.text().strip(),
                'timeout': 10
            }
            
            # Use password or private key
            if self.key_path_edit.text().strip():
                try:
                    # Try different key types
                    key_path = self.key_path_edit.text().strip()
                    private_key = None
                    
                    # Try RSA key first
                    try:
                        private_key = paramiko.RSAKey.from_private_key_file(key_path)
                    except Exception:
                        # Try Ed25519
                        try:
                            private_key = paramiko.Ed25519Key.from_private_key_file(key_path)
                        except Exception:
                            # Try ECDSA
                            try:
                                private_key = paramiko.ECDSAKey.from_private_key_file(key_path)
                            except Exception:
                                # Try DSS
                                try:
                                    private_key = paramiko.DSSKey.from_private_key_file(key_path)
                                except Exception as e:
                                    raise Exception(f"Unable to load private key: {str(e)}")
                    
                    connect_kwargs['pkey'] = private_key
                    
                except Exception as e:
                    self.test_btn.setEnabled(True)
                    self.test_btn.setText("Test Connection")
                    QMessageBox.critical(self, "Key Error", f"Failed to load private key:\n{str(e)}")
                    return
                    
            elif self.password_edit.text():
                connect_kwargs['password'] = self.password_edit.text()
            else:
                self.test_btn.setEnabled(True)
                self.test_btn.setText("Test Connection")
                QMessageBox.warning(self, "Authentication Error", "Please provide either password or private key.")
                return
            
            # Attempt connection
            ssh_client.connect(**connect_kwargs)
            
            # Test basic command execution
            stdin, stdout, stderr = ssh_client.exec_command('echo "Connection test successful"')
            result = stdout.read().decode().strip()
            
            ssh_client.close()
            
            self.test_btn.setEnabled(True)
            self.test_btn.setText("Test Connection")
            QMessageBox.information(self, "Connection Test", 
                                  f"✅ Connection successful!\n\nServer response: {result}")
            
        except paramiko.AuthenticationException:
            self.test_btn.setEnabled(True)
            self.test_btn.setText("Test Connection")
            QMessageBox.critical(self, "Authentication Failed", 
                               "Authentication failed. Please check your username, password, or private key.")
        except paramiko.SSHException as e:
            self.test_btn.setEnabled(True)
            self.test_btn.setText("Test Connection")
            QMessageBox.critical(self, "SSH Error", f"SSH connection failed:\n{str(e)}")
        except socket.timeout:
            self.test_btn.setEnabled(True)
            self.test_btn.setText("Test Connection")
            QMessageBox.critical(self, "Connection Timeout", 
                               "Connection timed out. Please check the host and port.")
        except socket.gaierror:
            self.test_btn.setEnabled(True)
            self.test_btn.setText("Test Connection")
            QMessageBox.critical(self, "DNS Error", 
                               "Unable to resolve hostname. Please check the host address.")
        except Exception as e:
            self.test_btn.setEnabled(True)
            self.test_btn.setText("Test Connection")
            QMessageBox.critical(self, "Connection Error", f"Connection failed:\n{str(e)}")
    
    def accept(self):
        # Validate inputs
        server_data = self.get_server_data()
        if not all([server_data.name, server_data.host, server_data.username]):
            QMessageBox.warning(self, "Validation Error", "Please fill in all required fields.")
            return
        
        super().accept()

class PortForwardDialog(QDialog):
    def __init__(self, parent=None, servers=None, port_forward=None):
        super().__init__(parent)
        self.servers = servers or []
        self.port_forward = port_forward
        self.is_edit = port_forward is not None
        self.setWindowTitle("Edit Port Forward" if self.is_edit else "Add Port Forward")
        self.setModal(True)
        self.setFixedSize(500, 400)
        self.setup_ui()
        
        if self.is_edit:
            self.load_port_forward_data()
    
    def setup_ui(self):
        layout = QVBoxLayout(self)
        
        # Basic info
        basic_group = QGroupBox("Basic Configuration")
        basic_layout = QFormLayout(basic_group)
        
        self.name_edit = QLineEdit()
        self.name_edit.setPlaceholderText("Port forward name")
        basic_layout.addRow("Name:", self.name_edit)
        
        self.server_combo = QComboBox()
        for server in self.servers:
            self.server_combo.addItem(server.name, server.id)
        basic_layout.addRow("SSH Server:", self.server_combo)
        
        # Direction selection
        direction_group = QGroupBox("Port Forward Direction")
        direction_layout = QVBoxLayout(direction_group)
        
        self.direction_combo = QComboBox()
        self.direction_combo.addItem("Local Forward (L → SSH → R)", "local")
        self.direction_combo.addItem("Remote Forward (R → SSH → L)", "remote") 
        self.direction_combo.addItem("Dynamic SOCKS Proxy", "dynamic")
        self.direction_combo.addItem("Reverse Dynamic SOCKS", "reverse-dynamic")
        self.direction_combo.currentTextChanged.connect(self.on_direction_changed)
        direction_layout.addWidget(self.direction_combo)
        
        # Help text
        self.direction_help = QLabel()
        self.direction_help.setWordWrap(True)
        self.direction_help.setStyleSheet("color: #666; font-size: 11px; padding: 5px;")
        direction_layout.addWidget(self.direction_help)
        
        # Local configuration
        local_group = QGroupBox("Local Configuration")
        local_layout = QFormLayout(local_group)
        
        self.local_host_edit = QLineEdit()
        self.local_host_edit.setText("127.0.0.1")
        self.local_host_edit.setPlaceholderText("Local bind address (127.0.0.1, 0.0.0.0, etc.)")
        local_layout.addRow("Local Host:", self.local_host_edit)
        
        self.local_port_spin = QSpinBox()
        self.local_port_spin.setRange(1, 65535)
        self.local_port_spin.setValue(8080)
        local_layout.addRow("Local Port:", self.local_port_spin)
        
        # Remote configuration
        self.remote_group = QGroupBox("Remote Configuration")
        remote_layout = QFormLayout(self.remote_group)
        
        self.remote_host_edit = QLineEdit()
        self.remote_host_edit.setPlaceholderText("Remote target host (localhost, 192.168.1.100, etc.)")
        self.remote_host_edit.setText("localhost")
        remote_layout.addRow("Remote Host:", self.remote_host_edit)
        
        self.remote_port_spin = QSpinBox()
        self.remote_port_spin.setRange(1, 65535)
        self.remote_port_spin.setValue(80)
        remote_layout.addRow("Remote Port:", self.remote_port_spin)
        
        # Options
        options_group = QGroupBox("Options")
        options_layout = QVBoxLayout(options_group)
        
        self.enabled_check = QCheckBox("Enabled")
        self.enabled_check.setChecked(True)
        options_layout.addWidget(self.enabled_check)
        
        self.auto_start_check = QCheckBox("Auto-start on application startup")
        options_layout.addWidget(self.auto_start_check)
        
        # Add all groups to main layout
        layout.addWidget(basic_group)
        layout.addWidget(direction_group)
        layout.addWidget(local_group)
        layout.addWidget(self.remote_group)
        layout.addWidget(options_group)
        
        # Buttons
        button_layout = QHBoxLayout()
        self.save_btn = QPushButton("Save")
        self.save_btn.clicked.connect(self.accept)
        self.cancel_btn = QPushButton("Cancel")
        self.cancel_btn.clicked.connect(self.reject)
        
        button_layout.addStretch()
        button_layout.addWidget(self.save_btn)
        button_layout.addWidget(self.cancel_btn)
        layout.addLayout(button_layout)
        
        # Set initial direction
        self.on_direction_changed()
    
    def on_direction_changed(self):
        """Handle direction change to update UI and help text"""
        direction = self.direction_combo.currentData()
        
        if direction == "local":
            self.direction_help.setText(
                "Local Forward: Connections to local host:port are forwarded through SSH to remote host:port.\n"
                "Example: Local web browser → localhost:8080 → SSH server → remote_host:80"
            )
            self.remote_group.setEnabled(True)
            self.remote_host_edit.setPlaceholderText("Target host on remote network")
            
        elif direction == "remote":
            self.direction_help.setText(
                "Remote Forward: Connections to SSH server's port are forwarded back to local host:port.\n"
                "Example: Remote users → SSH server:remote_port → SSH tunnel → local_host:local_port"
            )
            self.remote_group.setEnabled(True)
            self.remote_host_edit.setPlaceholderText("Local target host (usually localhost)")
            
        elif direction == "dynamic":
            self.direction_help.setText(
                "Dynamic SOCKS Proxy: Creates a SOCKS proxy server on local port.\n"
                "Applications can connect to localhost:local_port as a SOCKS proxy.\n"
                "Remote host/port settings are not used for SOCKS proxies."
            )
            self.remote_group.setEnabled(False)
            
        elif direction == "reverse-dynamic":
            self.direction_help.setText(
                "Reverse Dynamic SOCKS: Creates a SOCKS5 proxy on the SSH server that forwards to local network.\n"
                "Remote clients → SSH server:local_port (SOCKS proxy) → This local machine (via SSH tunnel)\n"
                "Remote clients can connect to any destination accessible from this local machine."
            )
            self.remote_group.setEnabled(False)  # Not used for reverse-dynamic
    
    def load_port_forward_data(self):
        self.name_edit.setText(self.port_forward.name)
        
        # Set server combo
        for i in range(self.server_combo.count()):
            if self.server_combo.itemData(i) == self.port_forward.server_id:
                self.server_combo.setCurrentIndex(i)
                break
        
        # Set direction
        for i in range(self.direction_combo.count()):
            if self.direction_combo.itemData(i) == self.port_forward.direction:
                self.direction_combo.setCurrentIndex(i)
                break
        
        self.local_host_edit.setText(self.port_forward.local_host)
        self.local_port_spin.setValue(self.port_forward.local_port)
        self.remote_host_edit.setText(self.port_forward.remote_host)
        self.remote_port_spin.setValue(self.port_forward.remote_port)
        self.enabled_check.setChecked(self.port_forward.enabled)
        self.auto_start_check.setChecked(self.port_forward.auto_start)
    
    def get_port_forward_data(self):
        return PortForward(
            id=self.port_forward.id if self.is_edit else None,
            server_id=self.server_combo.currentData(),
            name=self.name_edit.text().strip(),
            direction=self.direction_combo.currentData(),
            local_host=self.local_host_edit.text().strip(),
            local_port=self.local_port_spin.value(),
            remote_host=self.remote_host_edit.text().strip() if self.direction_combo.currentData() not in ["dynamic", "reverse-dynamic"] else "",
            remote_port=self.remote_port_spin.value() if self.direction_combo.currentData() not in ["dynamic", "reverse-dynamic"] else 0,
            enabled=self.enabled_check.isChecked(),
            auto_start=self.auto_start_check.isChecked()
        )
    
    def accept(self):
        # Validate inputs
        pf_data = self.get_port_forward_data()
        if not all([pf_data.name, pf_data.local_host]) or not pf_data.server_id:
            QMessageBox.warning(self, "Validation Error", "Please fill in all required fields.")
            return
        
        # Additional validation for non-dynamic forwards
        if pf_data.direction not in ["dynamic", "reverse-dynamic"] and not pf_data.remote_host:
            QMessageBox.warning(self, "Validation Error", "Remote host is required for this tunnel type.")
            return
        
        # Validate IP addresses
        import socket
        try:
            socket.inet_aton(pf_data.local_host)
        except socket.error:
            if pf_data.local_host not in ['localhost', '0.0.0.0']:
                QMessageBox.warning(self, "Validation Error", "Invalid local host address.")
                return
        
        super().accept()

class MainWindow(QMainWindow):
    def __init__(self):
        super().__init__()
        self.db = DatabaseManager()
        self.ssh_manager = SSHConnectionManager()
        
        # Initialize auto-start manager
        try:
            self.autostart_manager = AutoStartManager()
        except Exception as e:
            print(f"Warning: Auto-start manager initialization failed: {e}")
            self.autostart_manager = None
        
        self.setup_ui()
        self.setup_connections()
        self.load_data()
        
        # Update autostart menu state
        self.update_autostart_menu()
        
        # Status update timer
        self.status_timer = QTimer()
        self.status_timer.timeout.connect(self.update_status_display)
        self.status_timer.start(1000)  # Update every second
        
        # Auto-start port forwards marked for auto-start
        QTimer.singleShot(2000, self.auto_start_port_forwards)
    
    def setup_ui(self):
        self.setWindowTitle("SSH Port Forwarder")
        self.setGeometry(100, 100, 900, 600)
        
        # Central widget
        central_widget = QWidget()
        self.setCentralWidget(central_widget)
        
        # Main layout
        main_layout = QHBoxLayout(central_widget)
        
        # Splitter
        splitter = QSplitter(Qt.Horizontal)
        main_layout.addWidget(splitter)
        
        # Servers panel
        servers_widget = QWidget()
        servers_layout = QVBoxLayout(servers_widget)
        servers_layout.addWidget(QLabel("Servers"))
        
        # Servers table
        self.servers_table = QTableWidget()
        self.servers_table.setColumnCount(4)
        self.servers_table.setHorizontalHeaderLabels(["Name", "Host", "Port", "Username"])
        self.servers_table.horizontalHeader().setStretchLastSection(True)
        self.servers_table.setSelectionBehavior(QTableWidget.SelectRows)
        servers_layout.addWidget(self.servers_table)
        
        # Server buttons
        server_buttons = QHBoxLayout()
        self.add_server_btn = QPushButton("Add Server")
        self.edit_server_btn = QPushButton("Edit Server")
        self.delete_server_btn = QPushButton("Delete Server")
        server_buttons.addWidget(self.add_server_btn)
        server_buttons.addWidget(self.edit_server_btn)
        server_buttons.addWidget(self.delete_server_btn)
        server_buttons.addStretch()
        servers_layout.addLayout(server_buttons)
        
        # Port forwards panel
        pf_widget = QWidget()
        pf_layout = QVBoxLayout(pf_widget)
        pf_layout.addWidget(QLabel("Port Forwards"))
        
        # Port forwards table
        self.pf_table = QTableWidget()
        self.pf_table.setColumnCount(8)
        self.pf_table.setHorizontalHeaderLabels(["Name", "Server", "Direction", "Local", "Remote", "Status", "Enabled", "Auto Start"])
        self.pf_table.horizontalHeader().setStretchLastSection(True)
        self.pf_table.setSelectionBehavior(QTableWidget.SelectRows)
        self.pf_table.setEditTriggers(QTableWidget.NoEditTriggers)  # Make table uneditable
        self.pf_table.doubleClicked.connect(self.edit_port_forward)  # Double-click to edit
        pf_layout.addWidget(self.pf_table)
        
        # Port forward buttons
        pf_buttons = QHBoxLayout()
        self.add_pf_btn = QPushButton("Add Port Forward")
        self.edit_pf_btn = QPushButton("Edit")
        self.delete_pf_btn = QPushButton("Delete")
        self.toggle_pf_btn = QPushButton("Enable/Disable")
        self.start_pf_btn = QPushButton("Start")
        self.stop_pf_btn = QPushButton("Stop")
        self.start_all_btn = QPushButton("Start All")
        self.stop_all_btn = QPushButton("Stop All")
        
        # Style the Start All and Stop All buttons to make them stand out
        self.start_all_btn.setStyleSheet("QPushButton { background-color: #4CAF50; color: white; font-weight: bold; }")
        self.stop_all_btn.setStyleSheet("QPushButton { background-color: #f44336; color: white; font-weight: bold; }")
        
        pf_buttons.addWidget(self.add_pf_btn)
        pf_buttons.addWidget(self.edit_pf_btn)
        pf_buttons.addWidget(self.delete_pf_btn)
        pf_buttons.addWidget(self.toggle_pf_btn)
        pf_buttons.addWidget(self.start_pf_btn)
        pf_buttons.addWidget(self.stop_pf_btn)
        pf_buttons.addStretch()
        pf_buttons.addWidget(self.start_all_btn)
        pf_buttons.addWidget(self.stop_all_btn)
        pf_layout.addLayout(pf_buttons)
        
        # Add panels to splitter
        splitter.addWidget(servers_widget)
        splitter.addWidget(pf_widget)
        splitter.setStretchFactor(0, 1)
        splitter.setStretchFactor(1, 2)
        
        # Status bar
        self.statusBar().showMessage("Ready")
        
        # Menu bar
        self.create_menu_bar()
    
    def create_menu_bar(self):
        menubar = self.menuBar()
        
        # File menu
        file_menu = menubar.addMenu('File')
        
        exit_action = QAction('Exit', self)
        exit_action.triggered.connect(self.close)
        file_menu.addAction(exit_action)
        
        # Settings menu
        settings_menu = menubar.addMenu('Settings')
        
        self.autostart_action = QAction('Enable Auto-start', self)
        self.autostart_action.setCheckable(True)
        self.autostart_action.triggered.connect(self.toggle_autostart)
        settings_menu.addAction(self.autostart_action)
    
    def setup_connections(self):
        # Server buttons
        self.add_server_btn.clicked.connect(self.add_server)
        self.edit_server_btn.clicked.connect(self.edit_server)
        self.delete_server_btn.clicked.connect(self.delete_server)
        
        # Port forward buttons
        self.add_pf_btn.clicked.connect(self.add_port_forward)
        self.edit_pf_btn.clicked.connect(self.edit_port_forward)
        self.delete_pf_btn.clicked.connect(self.delete_port_forward)
        self.toggle_pf_btn.clicked.connect(self.toggle_port_forward)
        self.start_pf_btn.clicked.connect(self.start_port_forward)
        self.stop_pf_btn.clicked.connect(self.stop_port_forward)
        self.start_all_btn.clicked.connect(self.start_all_port_forwards)
        self.stop_all_btn.clicked.connect(self.stop_all_port_forwards)
        
        # SSH manager signals
        self.ssh_manager.tunnel_status_changed.connect(self.on_tunnel_status_changed)
        self.ssh_manager.tunnel_error.connect(self.on_tunnel_error)
        
        # Table selection changes
        self.servers_table.selectionModel().selectionChanged.connect(self.on_server_selection_changed)
        self.pf_table.selectionModel().selectionChanged.connect(self.on_pf_selection_changed)
    
    def load_data(self):
        self.load_servers()
        self.load_port_forwards()
    
    def load_servers(self):
        servers = self.db.get_servers()
        self.servers_table.setRowCount(len(servers))
        
        for row, server in enumerate(servers):
            self.servers_table.setItem(row, 0, QTableWidgetItem(server.name))
            self.servers_table.setItem(row, 1, QTableWidgetItem(server.host))
            self.servers_table.setItem(row, 2, QTableWidgetItem(str(server.port)))
            self.servers_table.setItem(row, 3, QTableWidgetItem(server.username))
            
            # Store server ID in first column
            self.servers_table.item(row, 0).setData(Qt.UserRole, server.id)
    
    def load_port_forwards(self):
        port_forwards = self.db.get_port_forwards()
        servers = {s.id: s for s in self.db.get_servers()}
        
        self.pf_table.setRowCount(len(port_forwards))
        
        for row, pf in enumerate(port_forwards):
            server_name = servers[pf.server_id].name if pf.server_id in servers else "Unknown"
            
            # Direction display
            direction_display = {
                'local': 'Local (L)',
                'remote': 'Remote (R)', 
                'dynamic': 'Dynamic (D)',
                'reverse-dynamic': 'Rev-Dynamic (RD)'
            }.get(pf.direction, 'Local (L)')
            
            self.pf_table.setItem(row, 0, QTableWidgetItem(pf.name))
            self.pf_table.setItem(row, 1, QTableWidgetItem(server_name))
            self.pf_table.setItem(row, 2, QTableWidgetItem(direction_display))
            self.pf_table.setItem(row, 3, QTableWidgetItem(f"{pf.local_host}:{pf.local_port}"))
            self.pf_table.setItem(row, 4, QTableWidgetItem(f"{pf.remote_host}:{pf.remote_port}" if pf.direction not in ['dynamic', 'reverse-dynamic'] else "N/A"))
            
            status = self.ssh_manager.get_tunnel_status(pf.id)
            self.pf_table.setItem(row, 5, QTableWidgetItem(status))
            self.pf_table.setItem(row, 6, QTableWidgetItem("Yes" if pf.enabled else "No"))
            self.pf_table.setItem(row, 7, QTableWidgetItem("Yes" if pf.auto_start else "No"))
            
            # Store port forward ID in first column
            self.pf_table.item(row, 0).setData(Qt.UserRole, pf.id)
    
    def update_status_display(self):
        """Update the status column of port forwards table"""
        for row in range(self.pf_table.rowCount()):
            pf_id = self.pf_table.item(row, 0).data(Qt.UserRole)
            if pf_id:
                status = self.ssh_manager.get_tunnel_status(pf_id)
                self.pf_table.setItem(row, 5, QTableWidgetItem(status))
                
                # Color code the status
                status_item = self.pf_table.item(row, 5)
                if status == 'connected':
                    status_item.setBackground(QColor(144, 238, 144))  # Light green
                elif status == 'connecting':
                    status_item.setBackground(QColor(255, 255, 0))   # Yellow
                elif status == 'error':
                    status_item.setBackground(QColor(255, 182, 193)) # Light red
                else:
                    status_item.setBackground(QColor(255, 255, 255)) # White
    
    def auto_start_port_forwards(self):
        """Start port forwards marked for auto-start"""
        auto_start_pfs = self.db.get_auto_start_port_forwards()
        servers = {s.id: s for s in self.db.get_servers()}
        
        for pf in auto_start_pfs:
            if pf.server_id in servers:
                self.ssh_manager.start_tunnel(servers[pf.server_id], pf)
                self.statusBar().showMessage(f"Auto-started: {pf.name}")
    
    def add_server(self):
        dialog = ServerDialog(self)
        if dialog.exec_() == QDialog.Accepted:
            server_data = dialog.get_server_data()
            try:
                server_id = self.db.add_server(server_data)
                self.load_servers()
                self.statusBar().showMessage(f"Server '{server_data.name}' added successfully")
            except Exception as e:
                QMessageBox.critical(self, "Error", f"Failed to add server: {str(e)}")
    
    def edit_server(self):
        current_row = self.servers_table.currentRow()
        if current_row < 0:
            QMessageBox.information(self, "No Selection", "Please select a server to edit.")
            return
        
        server_id = self.servers_table.item(current_row, 0).data(Qt.UserRole)
        server = self.db.get_server(server_id)
        
        dialog = ServerDialog(self, server)
        if dialog.exec_() == QDialog.Accepted:
            server_data = dialog.get_server_data()
            try:
                self.db.update_server(server_data)
                self.load_servers()
                self.statusBar().showMessage(f"Server '{server_data.name}' updated successfully")
            except Exception as e:
                QMessageBox.critical(self, "Error", f"Failed to update server: {str(e)}")
    
    def delete_server(self):
        current_row = self.servers_table.currentRow()
        if current_row < 0:
            QMessageBox.information(self, "No Selection", "Please select a server to delete.")
            return
        
        server_name = self.servers_table.item(current_row, 0).text()
        server_id = self.servers_table.item(current_row, 0).data(Qt.UserRole)
        
        reply = QMessageBox.question(self, "Confirm Delete", 
                                   f"Are you sure you want to delete server '{server_name}'?\n"
                                   "This will also delete all associated port forwards.",
                                   QMessageBox.Yes | QMessageBox.No)
        
        if reply == QMessageBox.Yes:
            try:
                self.db.delete_server(server_id)
                self.load_data()
                self.statusBar().showMessage(f"Server '{server_name}' deleted successfully")
            except Exception as e:
                QMessageBox.critical(self, "Error", f"Failed to delete server: {str(e)}")
    
    def add_port_forward(self):
        servers = self.db.get_servers()
        if not servers:
            QMessageBox.information(self, "No Servers", "Please add at least one server first.")
            return
        
        dialog = PortForwardDialog(self, servers)
        if dialog.exec_() == QDialog.Accepted:
            pf_data = dialog.get_port_forward_data()
            try:
                pf_id = self.db.add_port_forward(pf_data)
                self.load_port_forwards()
                self.statusBar().showMessage(f"Port forward '{pf_data.name}' added successfully")
            except Exception as e:
                QMessageBox.critical(self, "Error", f"Failed to add port forward: {str(e)}")
    
    def edit_port_forward(self):
        current_row = self.pf_table.currentRow()
        if current_row < 0:
            QMessageBox.information(self, "No Selection", "Please select a port forward to edit.")
            return
        
        pf_id = self.pf_table.item(current_row, 0).data(Qt.UserRole)
        pf = self.db.get_port_forward(pf_id)
        servers = self.db.get_servers()
        
        dialog = PortForwardDialog(self, servers, pf)
        if dialog.exec_() == QDialog.Accepted:
            pf_data = dialog.get_port_forward_data()
            try:
                self.db.update_port_forward(pf_data)
                self.load_port_forwards()
                self.statusBar().showMessage(f"Port forward '{pf_data.name}' updated successfully")
            except Exception as e:
                QMessageBox.critical(self, "Error", f"Failed to update port forward: {str(e)}")
    
    def delete_port_forward(self):
        current_row = self.pf_table.currentRow()
        if current_row < 0:
            QMessageBox.information(self, "No Selection", "Please select a port forward to delete.")
            return
        
        pf_name = self.pf_table.item(current_row, 0).text()
        pf_id = self.pf_table.item(current_row, 0).data(Qt.UserRole)
        
        reply = QMessageBox.question(self, "Confirm Delete", 
                                   f"Are you sure you want to delete port forward '{pf_name}'?",
                                   QMessageBox.Yes | QMessageBox.No)
        
        if reply == QMessageBox.Yes:
            try:
                # Stop tunnel if active
                self.ssh_manager.stop_tunnel(pf_id)
                self.db.delete_port_forward(pf_id)
                self.load_port_forwards()
                self.statusBar().showMessage(f"Port forward '{pf_name}' deleted successfully")
            except Exception as e:
                QMessageBox.critical(self, "Error", f"Failed to delete port forward: {str(e)}")
    
    def toggle_port_forward(self):
        current_row = self.pf_table.currentRow()
        if current_row < 0:
            QMessageBox.information(self, "No Selection", "Please select a port forward to toggle.")
            return
        
        pf_id = self.pf_table.item(current_row, 0).data(Qt.UserRole)
        try:
            self.db.toggle_port_forward(pf_id)
            self.load_port_forwards()
            self.statusBar().showMessage("Port forward toggled")
        except Exception as e:
            QMessageBox.critical(self, "Error", f"Failed to toggle port forward: {str(e)}")
    
    def start_port_forward(self):
        current_row = self.pf_table.currentRow()
        if current_row < 0:
            QMessageBox.information(self, "No Selection", "Please select a port forward to start.")
            return
        
        pf_id = self.pf_table.item(current_row, 0).data(Qt.UserRole)
        pf = self.db.get_port_forward(pf_id)
        server = self.db.get_server(pf.server_id)
        
        if not pf.enabled:
            QMessageBox.information(self, "Port Forward Disabled", 
                                  "This port forward is disabled. Please enable it first.")
            return
        
        # Check if port is already in use
        import socket
        test_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        try:
            test_socket.bind(('127.0.0.1', pf.local_port))
            test_socket.close()
        except OSError:
            reply = QMessageBox.question(self, "Port In Use", 
                                       f"Port {pf.local_port} is already in use. "
                                       f"Do you want to try starting the tunnel anyway?\n\n"
                                       f"This might work if another tunnel is using the same port.",
                                       QMessageBox.Yes | QMessageBox.No)
            if reply == QMessageBox.No:
                return
        
        try:
            self.ssh_manager.start_tunnel(server, pf)
            self.statusBar().showMessage(f"Starting tunnel: {pf.name} (localhost:{pf.local_port} -> {pf.remote_host}:{pf.remote_port})")
        except Exception as e:
            QMessageBox.critical(self, "Error", f"Failed to start port forward: {str(e)}")
    
    def stop_port_forward(self):
        current_row = self.pf_table.currentRow()
        if current_row < 0:
            QMessageBox.information(self, "No Selection", "Please select a port forward to stop.")
            return
        
        pf_id = self.pf_table.item(current_row, 0).data(Qt.UserRole)
        pf_name = self.pf_table.item(current_row, 0).text()
        
        try:
            self.ssh_manager.stop_tunnel(pf_id)
            self.statusBar().showMessage(f"Stopped tunnel: {pf_name}")
        except Exception as e:
            QMessageBox.critical(self, "Error", f"Failed to stop port forward: {str(e)}")
    
    def start_all_port_forwards(self):
        """Start all enabled port forwards"""
        port_forwards = self.db.get_port_forwards()
        enabled_pfs = [pf for pf in port_forwards if pf.enabled]
        
        if not enabled_pfs:
            QMessageBox.information(self, "No Port Forwards", "No enabled port forwards to start.")
            return
        
        started_count = 0
        failed_count = 0
        port_conflicts = []
        
        # Check for port conflicts first
        import socket
        for pf in enabled_pfs:
            test_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            try:
                test_socket.bind(('127.0.0.1', pf.local_port))
                test_socket.close()
            except OSError:
                port_conflicts.append(f"{pf.name} (port {pf.local_port})")
        
        # Show warning if there are port conflicts
        if port_conflicts:
            conflicts_text = "\\n".join(port_conflicts)
            reply = QMessageBox.question(self, "Port Conflicts Detected", 
                                       f"The following port forwards have port conflicts:\\n\\n{conflicts_text}\\n\\n"
                                       f"Do you want to continue starting all enabled port forwards?\\n"
                                       f"Conflicted tunnels may fail to start.",
                                       QMessageBox.Yes | QMessageBox.No)
            if reply == QMessageBox.No:
                return
        
        # Start all enabled port forwards
        for pf in enabled_pfs:
            try:
                server = self.db.get_server(pf.server_id)
                self.ssh_manager.start_tunnel(server, pf)
                started_count += 1
            except Exception as e:
                failed_count += 1
                print(f"Failed to start {pf.name}: {e}")
        
        self.statusBar().showMessage(f"Started {started_count} tunnels, {failed_count} failed")
        
        if failed_count > 0:
            QMessageBox.warning(self, "Partial Success", 
                              f"Started {started_count} port forwards successfully.\\n"
                              f"{failed_count} port forwards failed to start. Check the log for details.")
    
    def stop_all_port_forwards(self):
        """Stop all active port forwards"""
        # Get all active tunnels from the SSH manager
        active_tunnel_ids = list(self.ssh_manager.active_tunnels.keys())
        
        if not active_tunnel_ids:
            QMessageBox.information(self, "No Active Tunnels", "No active tunnels to stop.")
            return
        
        stopped_count = 0
        failed_count = 0
        
        for pf_id in active_tunnel_ids:
            try:
                self.ssh_manager.stop_tunnel(pf_id)
                stopped_count += 1
            except Exception as e:
                failed_count += 1
                print(f"Failed to stop tunnel {pf_id}: {e}")
        
        self.statusBar().showMessage(f"Stopped {stopped_count} tunnels, {failed_count} failed")
        
        if failed_count > 0:
            QMessageBox.warning(self, "Partial Success", 
                              f"Stopped {stopped_count} tunnels successfully.\\n"
                              f"{failed_count} tunnels failed to stop.")
    
    def on_tunnel_status_changed(self, pf_id, status):
        self.statusBar().showMessage(f"Tunnel {pf_id} status: {status}")
    
    def on_tunnel_error(self, pf_id, error):
        QMessageBox.warning(self, "Tunnel Error", f"Tunnel {pf_id} error: {error}")
    
    def on_server_selection_changed(self):
        has_selection = self.servers_table.currentRow() >= 0
        self.edit_server_btn.setEnabled(has_selection)
        self.delete_server_btn.setEnabled(has_selection)
    
    def on_pf_selection_changed(self):
        has_selection = self.pf_table.currentRow() >= 0
        self.edit_pf_btn.setEnabled(has_selection)
        self.delete_pf_btn.setEnabled(has_selection)
        self.toggle_pf_btn.setEnabled(has_selection)
        self.start_pf_btn.setEnabled(has_selection)
        self.stop_pf_btn.setEnabled(has_selection)
    
    def update_autostart_menu(self):
        """Update the autostart menu item based on current state"""
        if self.autostart_manager:
            is_enabled = self.autostart_manager.is_auto_start_enabled()
            self.autostart_action.setChecked(is_enabled)
            self.autostart_action.setText("Disable Auto-start" if is_enabled else "Enable Auto-start")
    
    def toggle_autostart(self):
        """Toggle Windows auto-start functionality"""
        if not self.autostart_manager:
            QMessageBox.warning(self, "Auto-start Unavailable", 
                              "Auto-start functionality is not available on this system.")
            return
        
        try:
            if self.autostart_manager.is_auto_start_enabled():
                self.autostart_manager.disable_auto_start()
                QMessageBox.information(self, "Auto-start Disabled", 
                                      "Application will no longer start automatically at Windows boot.")
            else:
                self.autostart_manager.enable_auto_start()
                QMessageBox.information(self, "Auto-start Enabled", 
                                      "Application will now start automatically at Windows boot.")
            
            # Update the menu item
            self.update_autostart_menu()
            
        except Exception as e:
            QMessageBox.critical(self, "Auto-start Error", f"Failed to toggle auto-start:\n{str(e)}")
    
    def closeEvent(self, event):
        # Stop all active tunnels
        self.ssh_manager.stop_all_tunnels()
        event.accept()