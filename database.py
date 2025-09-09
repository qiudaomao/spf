import sqlite3
import os
from typing import Dict, List, Optional
from dataclasses import dataclass
from datetime import datetime

@dataclass
class Server:
    id: Optional[int]
    name: str
    host: str
    port: int
    username: str
    password: Optional[str]
    private_key_path: Optional[str]
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None

@dataclass
class PortForward:
    id: Optional[int]
    server_id: int
    name: str
    direction: str  # 'local', 'remote', 'dynamic'
    local_host: str
    local_port: int
    remote_host: str
    remote_port: int
    enabled: bool = True
    auto_start: bool = False
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None

class DatabaseManager:
    def __init__(self, db_path: str = "ssh_port_forwarder.db"):
        self.db_path = db_path
        self.init_database()
    
    def init_database(self):
        """Initialize the database with required tables"""
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        # Create servers table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS servers (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                name TEXT NOT NULL UNIQUE,
                host TEXT NOT NULL,
                port INTEGER NOT NULL DEFAULT 22,
                username TEXT NOT NULL,
                password TEXT,
                private_key_path TEXT,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
        ''')
        
        # Create port_forwards table with enhanced schema
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS port_forwards (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                server_id INTEGER NOT NULL,
                name TEXT NOT NULL,
                direction TEXT NOT NULL DEFAULT 'local',
                local_host TEXT NOT NULL DEFAULT '127.0.0.1',
                local_port INTEGER NOT NULL,
                remote_host TEXT NOT NULL,
                remote_port INTEGER NOT NULL,
                enabled BOOLEAN DEFAULT TRUE,
                auto_start BOOLEAN DEFAULT FALSE,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                FOREIGN KEY (server_id) REFERENCES servers (id) ON DELETE CASCADE,
                UNIQUE(direction, local_host, local_port)
            )
        ''')
        
        # Check if we need to migrate existing data
        self._migrate_port_forwards_table(cursor)
        
        # Create indexes
        cursor.execute('CREATE INDEX IF NOT EXISTS idx_port_forwards_server_id ON port_forwards(server_id)')
        cursor.execute('CREATE INDEX IF NOT EXISTS idx_port_forwards_local ON port_forwards(direction, local_host, local_port)')
        
        conn.commit()
        conn.close()
    
    def _migrate_port_forwards_table(self, cursor):
        """Migrate existing port_forwards table to new schema if needed"""
        try:
            # Check if old schema exists (missing direction and local_host columns)
            cursor.execute("PRAGMA table_info(port_forwards)")
            columns = [column[1] for column in cursor.fetchall()]
            
            needs_migration = False
            if 'direction' not in columns:
                cursor.execute('ALTER TABLE port_forwards ADD COLUMN direction TEXT NOT NULL DEFAULT "local"')
                needs_migration = True
                
            if 'local_host' not in columns:
                cursor.execute('ALTER TABLE port_forwards ADD COLUMN local_host TEXT NOT NULL DEFAULT "127.0.0.1"')
                needs_migration = True
            
            if needs_migration:
                print("Migrated port_forwards table to new schema")
                
        except sqlite3.Error as e:
            print(f"Migration warning: {e}")
    
    def add_server(self, server: Server) -> int:
        """Add a new server and return its ID"""
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        cursor.execute('''
            INSERT INTO servers (name, host, port, username, password, private_key_path)
            VALUES (?, ?, ?, ?, ?, ?)
        ''', (server.name, server.host, server.port, server.username, 
              server.password, server.private_key_path))
        
        server_id = cursor.lastrowid
        conn.commit()
        conn.close()
        return server_id
    
    def update_server(self, server: Server) -> bool:
        """Update an existing server"""
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        cursor.execute('''
            UPDATE servers SET 
                name = ?, host = ?, port = ?, username = ?, 
                password = ?, private_key_path = ?, updated_at = CURRENT_TIMESTAMP
            WHERE id = ?
        ''', (server.name, server.host, server.port, server.username,
              server.password, server.private_key_path, server.id))
        
        success = cursor.rowcount > 0
        conn.commit()
        conn.close()
        return success
    
    def delete_server(self, server_id: int) -> bool:
        """Delete a server and all its port forwards"""
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        cursor.execute('DELETE FROM servers WHERE id = ?', (server_id,))
        success = cursor.rowcount > 0
        conn.commit()
        conn.close()
        return success
    
    def get_servers(self) -> List[Server]:
        """Get all servers"""
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        
        cursor.execute('SELECT * FROM servers ORDER BY name')
        rows = cursor.fetchall()
        conn.close()
        
        servers = []
        for row in rows:
            servers.append(Server(
                id=row['id'],
                name=row['name'],
                host=row['host'],
                port=row['port'],
                username=row['username'],
                password=row['password'],
                private_key_path=row['private_key_path'],
                created_at=datetime.fromisoformat(row['created_at']) if row['created_at'] else None,
                updated_at=datetime.fromisoformat(row['updated_at']) if row['updated_at'] else None
            ))
        return servers
    
    def get_server(self, server_id: int) -> Optional[Server]:
        """Get a specific server by ID"""
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        
        cursor.execute('SELECT * FROM servers WHERE id = ?', (server_id,))
        row = cursor.fetchone()
        conn.close()
        
        if row:
            return Server(
                id=row['id'],
                name=row['name'],
                host=row['host'],
                port=row['port'],
                username=row['username'],
                password=row['password'],
                private_key_path=row['private_key_path'],
                created_at=datetime.fromisoformat(row['created_at']) if row['created_at'] else None,
                updated_at=datetime.fromisoformat(row['updated_at']) if row['updated_at'] else None
            )
        return None
    
    def add_port_forward(self, port_forward: PortForward) -> int:
        """Add a new port forward and return its ID"""
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        cursor.execute('''
            INSERT INTO port_forwards (server_id, name, direction, local_host, local_port, remote_host, remote_port, enabled, auto_start)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        ''', (port_forward.server_id, port_forward.name, port_forward.direction,
              port_forward.local_host, port_forward.local_port, port_forward.remote_host, 
              port_forward.remote_port, port_forward.enabled, port_forward.auto_start))
        
        pf_id = cursor.lastrowid
        conn.commit()
        conn.close()
        return pf_id
    
    def update_port_forward(self, port_forward: PortForward) -> bool:
        """Update an existing port forward"""
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        cursor.execute('''
            UPDATE port_forwards SET 
                server_id = ?, name = ?, direction = ?, local_host = ?, local_port = ?, 
                remote_host = ?, remote_port = ?, enabled = ?, auto_start = ?, updated_at = CURRENT_TIMESTAMP
            WHERE id = ?
        ''', (port_forward.server_id, port_forward.name, port_forward.direction,
              port_forward.local_host, port_forward.local_port, port_forward.remote_host, 
              port_forward.remote_port, port_forward.enabled, port_forward.auto_start, port_forward.id))
        
        success = cursor.rowcount > 0
        conn.commit()
        conn.close()
        return success
    
    def delete_port_forward(self, pf_id: int) -> bool:
        """Delete a port forward"""
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        cursor.execute('DELETE FROM port_forwards WHERE id = ?', (pf_id,))
        success = cursor.rowcount > 0
        conn.commit()
        conn.close()
        return success
    
    def get_port_forwards(self, server_id: Optional[int] = None) -> List[PortForward]:
        """Get all port forwards or port forwards for a specific server"""
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        
        if server_id:
            cursor.execute('SELECT * FROM port_forwards WHERE server_id = ? ORDER BY name', (server_id,))
        else:
            cursor.execute('SELECT * FROM port_forwards ORDER BY name')
        
        rows = cursor.fetchall()
        conn.close()
        
        port_forwards = []
        for row in rows:
            port_forwards.append(PortForward(
                id=row['id'],
                server_id=row['server_id'],
                name=row['name'],
                direction=row['direction'],  # Handle migration case
                local_host=row['local_host'],  # Handle migration case
                local_port=row['local_port'],
                remote_host=row['remote_host'],
                remote_port=row['remote_port'],
                enabled=bool(row['enabled']),
                auto_start=bool(row['auto_start']),
                created_at=datetime.fromisoformat(row['created_at']) if row['created_at'] else None,
                updated_at=datetime.fromisoformat(row['updated_at']) if row['updated_at'] else None
            ))
        return port_forwards
    
    def get_port_forward(self, pf_id: int) -> Optional[PortForward]:
        """Get a specific port forward by ID"""
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        
        cursor.execute('SELECT * FROM port_forwards WHERE id = ?', (pf_id,))
        row = cursor.fetchone()
        conn.close()
        
        if row:
            return PortForward(
                id=row['id'],
                server_id=row['server_id'],
                name=row['name'],
                direction=row['direction'],  # Handle migration case
                local_host=row['local_host'],  # Handle migration case
                local_port=row['local_port'],
                remote_host=row['remote_host'],
                remote_port=row['remote_port'],
                enabled=bool(row['enabled']),
                auto_start=bool(row['auto_start']),
                created_at=datetime.fromisoformat(row['created_at']) if row['created_at'] else None,
                updated_at=datetime.fromisoformat(row['updated_at']) if row['updated_at'] else None
            )
        return None
    
    def toggle_port_forward(self, pf_id: int) -> bool:
        """Toggle the enabled state of a port forward"""
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        cursor.execute('''
            UPDATE port_forwards SET 
                enabled = NOT enabled, updated_at = CURRENT_TIMESTAMP
            WHERE id = ?
        ''', (pf_id,))
        
        success = cursor.rowcount > 0
        conn.commit()
        conn.close()
        return success
    
    def get_auto_start_port_forwards(self) -> List[PortForward]:
        """Get all port forwards marked for auto-start"""
        return [pf for pf in self.get_port_forwards() if pf.auto_start and pf.enabled]