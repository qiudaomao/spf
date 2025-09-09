#!/usr/bin/env python3
"""
Test script to validate SSH port forwarding functionality
"""

import socket
import time
import threading
import requests
from http.server import HTTPServer, SimpleHTTPRequestHandler
import os

def test_local_server(port=8000):
    """Start a simple HTTP server for testing"""
    class TestHandler(SimpleHTTPRequestHandler):
        def do_GET(self):
            self.send_response(200)
            self.send_header('Content-type', 'text/plain')
            self.end_headers()
            self.wfile.write(b'SSH Port Forward Test - Connection Successful!')
    
    try:
        server = HTTPServer(('localhost', port), TestHandler)
        print(f"Starting test HTTP server on port {port}")
        server.serve_forever()
    except OSError as e:
        print(f"Failed to start test server on port {port}: {e}")

def test_port_connection(port, expected_response=None):
    """Test if a port is accessible and optionally check response"""
    try:
        # Test basic port connectivity
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(5)
        result = sock.connect_ex(('localhost', port))
        sock.close()
        
        if result == 0:
            print(f"✅ Port {port} is accessible")
            
            # If it's an HTTP port, try to get content
            if expected_response:
                try:
                    response = requests.get(f'http://localhost:{port}', timeout=5)
                    if expected_response in response.text:
                        print(f"✅ HTTP response contains expected content")
                        return True
                    else:
                        print(f"❌ HTTP response does not contain expected content")
                        print(f"Got: {response.text}")
                        return False
                except Exception as e:
                    print(f"❌ HTTP request failed: {e}")
                    return False
            return True
        else:
            print(f"❌ Port {port} is not accessible")
            return False
            
    except Exception as e:
        print(f"❌ Error testing port {port}: {e}")
        return False

def main():
    """Main test function"""
    print("SSH Port Forwarder Test Script")
    print("=" * 40)
    
    # Test 1: Check if common local ports are available
    print("\n1. Testing local port availability:")
    test_ports = [8080, 8081, 8082, 9000]
    
    for port in test_ports:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        try:
            sock.bind(('localhost', port))
            sock.close()
            print(f"✅ Port {port} is available")
        except OSError:
            print(f"❌ Port {port} is in use")
    
    # Test 2: Start a test HTTP server in background
    print("\n2. Starting test HTTP server on port 8000...")
    server_thread = threading.Thread(target=test_local_server, args=(8000,), daemon=True)
    server_thread.start()
    time.sleep(1)  # Give server time to start
    
    # Test 3: Verify test server is running
    print("\n3. Testing HTTP server:")
    if test_port_connection(8000, "SSH Port Forward Test"):
        print("✅ Test HTTP server is working correctly")
    else:
        print("❌ Test HTTP server is not responding")
    
    # Test 4: Instructions for manual testing
    print("\n4. Manual Testing Instructions:")
    print("-" * 30)
    print("1. Start the SSH Port Forwarder application")
    print("2. Add an SSH server configuration")
    print("3. Create a port forward rule:")
    print("   - Local Port: 8080 (or any available port)")
    print("   - Remote Host: localhost")
    print("   - Remote Port: 8000")
    print("4. Start the port forward")
    print("5. Test the tunnel by visiting: http://localhost:8080")
    print("   You should see: 'SSH Port Forward Test - Connection Successful!'")
    
    # Test 5: Show common troubleshooting
    print("\n5. Common Issues and Solutions:")
    print("-" * 30)
    print("• Port already in use: Choose a different local port")
    print("• SSH authentication fails: Check username/password/key")
    print("• Connection timeout: Check SSH server is accessible")
    print("• Permission denied: Use ports > 1024 for local binding")
    print("• Tunnel shows 'connected' but doesn't work:")
    print("  - Check remote host/port are correct")
    print("  - Verify SSH server allows port forwarding")
    print("  - Check firewall settings on remote server")
    
    # Test 6: Keep server running for manual tests
    print(f"\n6. Test server is running on http://localhost:8000")
    print("Press Ctrl+C to stop...")
    
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print("\nStopping test server...")

if __name__ == "__main__":
    main()