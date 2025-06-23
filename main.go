//go:build !windows
// +build !windows

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/ini.v1"
)

type ServerConfig struct {
	Server   string
	User     string
	Password string
	Port     string
}

type CommonConfig struct {
	Debug bool
}

type ForwardConfig struct {
	SectionName string
	ServerName  string
	RemoteIP    string
	RemotePort  string
	LocalIP     string
	LocalPort   string
	Direction   string
	SSHConfig   *ServerConfig
	// SOCKS5 authentication
	Socks5User string
	Socks5Pass string
}

// Connection manager for shared SSH connections
type ConnectionManager struct {
	connections map[string]*ssh.Client
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

var (
	connManager *ConnectionManager
	servers     map[string]*ServerConfig
	ctx         context.Context
	cancel      context.CancelFunc
)

func main() {
	// Initialize context for graceful shutdown
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Initialize connection manager
	connManager = &ConnectionManager{
		connections: make(map[string]*ssh.Client),
		ctx:         ctx,
		cancel:      cancel,
	}

	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	// Parse common configuration
	var commonConfig CommonConfig
	if cfg.HasSection("common") {
		commonSection := cfg.Section("common")
		commonConfig.Debug = commonSection.Key("debug").MustBool(false)
	}

	servers = make(map[string]*ServerConfig)
	var forwardConfigs []*ForwardConfig

	for _, section := range cfg.Sections() {
		if section.Name() == "DEFAULT" || section.Name() == "common" {
			continue
		}

		if section.HasKey("user") && section.HasKey("password") {
			port := section.Key("port").String()
			if port == "" {
				port = "22" // Default SSH port
			}
			servers[section.Name()] = &ServerConfig{
				Server:   section.Key("server").String(),
				User:     section.Key("user").String(),
				Password: section.Key("password").String(),
				Port:     port,
			}
		} else if section.HasKey("server") && section.HasKey("direction") {
			forwardConfig := &ForwardConfig{
				SectionName: section.Name(),
				ServerName:  section.Key("server").String(),
				RemoteIP:    section.Key("remoteIP").String(),
				RemotePort:  section.Key("remotePort").String(),
				LocalIP:     section.Key("localIP").String(),
				LocalPort:   section.Key("localPort").String(),
				Direction:   section.Key("direction").String(),
				Socks5User:  section.Key("socks5User").String(),
				Socks5Pass:  section.Key("socks5Pass").String(),
			}
			forwardConfigs = append(forwardConfigs, forwardConfig)
		}
	}

	for _, fc := range forwardConfigs {
		if sshConfig, ok := servers[fc.ServerName]; ok {
			fc.SSHConfig = sshConfig
			go handleConnection(fc, &commonConfig)
		} else {
			log.Printf("Warning: No server configuration found for %s", fc.SectionName)
		}
	}

	// Keep the main goroutine running
	select {}
}

func handleConnection(config *ForwardConfig, commonConfig *CommonConfig) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := connectAndForward(config, commonConfig)
			if err != nil {
				log.Printf("Error in connection for %s: %v. Retrying in 30 seconds...", config.SectionName, err)

				// Remove the failed connection so it can be recreated
				connManager.RemoveConnection(config.ServerName)

				select {
				case <-time.After(30 * time.Second):
					continue
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func connectAndForward(config *ForwardConfig, commonConfig *CommonConfig) error {
	// Get shared SSH connection
	conn, err := connManager.GetConnection(config.ServerName)
	if err != nil {
		return fmt.Errorf("failed to get connection for %s: %v", config.ServerName, err)
	}

	log.Printf("Using shared connection to %s for %s", config.SSHConfig.Server, config.SectionName)

	switch config.Direction {
	case "remote":
		err = handleRemotePortForward(conn, config, commonConfig)
	case "local":
		err = handleLocalPortForward(conn, config, commonConfig)
	case "socks5":
		err = handleSocks5Proxy(conn, config, commonConfig)
	case "reverse-socks5":
		err = handleReverseSocks5Proxy(conn, config, commonConfig)
	default:
		return fmt.Errorf("invalid direction: %s", config.Direction)
	}

	return err
}

func handleRemotePortForward(conn *ssh.Client, config *ForwardConfig, commonConfig *CommonConfig) error {
	listener, err := conn.Listen("tcp", fmt.Sprintf("%s:%s", config.RemoteIP, config.RemotePort))
	if err != nil {
		return fmt.Errorf("failed to listen on remote server: %v", err)
	}
	defer listener.Close()

	log.Printf("Listening on %s:%s for remote port forwarding", config.RemoteIP, config.RemotePort)

	for {
		remoteConn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection: %v", err)
		}

		go handleForwardingConnection(remoteConn, config.LocalIP, config.LocalPort, commonConfig)
	}
}

func handleLocalPortForward(conn *ssh.Client, config *ForwardConfig, commonConfig *CommonConfig) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", config.LocalIP, config.LocalPort))
	if err != nil {
		return fmt.Errorf("failed to listen on local address: %v", err)
	}
	defer listener.Close()

	log.Printf("Listening on %s:%s for local port forwarding", config.LocalIP, config.LocalPort)

	for {
		localConn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection: %v", err)
		}

		go func() {
			remoteConn, err := conn.Dial("tcp", fmt.Sprintf("%s:%s", config.RemoteIP, config.RemotePort))
			if err != nil {
				log.Printf("Failed to connect to remote address: %v", err)
				localConn.Close()
				return
			}

			go copyConn(localConn, remoteConn, commonConfig)
			go copyConn(remoteConn, localConn, commonConfig)
		}()
	}
}

func handleForwardingConnection(incomingConn net.Conn, targetIP, targetPort string, commonConfig *CommonConfig) {
	targetConn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", targetIP, targetPort))
	if err != nil {
		log.Printf("Failed to connect to target address: %v", err)
		incomingConn.Close()
		return
	}

	go copyConn(targetConn, incomingConn, commonConfig)
	go copyConn(incomingConn, targetConn, commonConfig)
}

func handleSocks5Proxy(conn *ssh.Client, config *ForwardConfig, commonConfig *CommonConfig) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", config.LocalIP, config.LocalPort))
	if err != nil {
		return fmt.Errorf("failed to listen on local address: %v", err)
	}
	defer listener.Close()

	log.Printf("SOCKS5 proxy listening on %s:%s", config.LocalIP, config.LocalPort)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection: %v", err)
		}

		go handleSocks5Connection(clientConn, conn, config, commonConfig)
	}
}

func handleSocks5Connection(clientConn net.Conn, sshConn *ssh.Client, config *ForwardConfig, commonConfig *CommonConfig) {
	defer clientConn.Close()

	// Create a SOCKS5 server that uses the SSH connection for dialing
	socks5Server := &socks5Server{
		sshConn: sshConn,
		config:  config,
	}

	// Handle the SOCKS5 protocol
	err := socks5Server.handleConnection(clientConn, commonConfig)
	if err != nil {
		log.Printf("SOCKS5 connection error: %v", err)
	}
}

type socks5Server struct {
	sshConn *ssh.Client
	config  *ForwardConfig
}

func (s *socks5Server) handleConnection(clientConn net.Conn, commonConfig *CommonConfig) error {
	// Read SOCKS5 version and number of authentication methods
	buf := make([]byte, 256)
	n, err := clientConn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read SOCKS5 greeting: %v", err)
	}

	if n < 2 || buf[0] != 0x05 {
		return fmt.Errorf("invalid SOCKS5 version")
	}

	// Check if authentication is required
	requireAuth := s.config.Socks5User != "" && s.config.Socks5Pass != ""

	// Parse supported authentication methods
	numMethods := int(buf[1])
	if n < 2+numMethods {
		return fmt.Errorf("invalid authentication methods")
	}

	supportedMethods := buf[2 : 2+numMethods]
	var selectedMethod byte = 0xFF // No acceptable methods

	if requireAuth {
		// Check if client supports username/password authentication (method 0x02)
		for _, method := range supportedMethods {
			if method == 0x02 {
				selectedMethod = 0x02
				break
			}
		}
	} else {
		// Check if client supports no authentication (method 0x00)
		for _, method := range supportedMethods {
			if method == 0x00 {
				selectedMethod = 0x00
				break
			}
		}
	}

	// Send authentication method selection response
	_, err = clientConn.Write([]byte{0x05, selectedMethod})
	if err != nil {
		return fmt.Errorf("failed to send auth method response: %v", err)
	}

	if selectedMethod == 0xFF {
		return fmt.Errorf("no acceptable authentication methods")
	}

	// Handle authentication if required
	if selectedMethod == 0x02 {
		err = s.handleUsernamePasswordAuth(clientConn, commonConfig)
		if err != nil {
			return fmt.Errorf("authentication failed: %v", err)
		}
	}

	// Read connection request
	n, err = clientConn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read connection request: %v", err)
	}

	if n < 4 || buf[0] != 0x05 || buf[1] != 0x01 {
		return fmt.Errorf("invalid SOCKS5 connection request")
	}

	// Parse target address
	var targetAddr string
	var targetPort uint16

	switch buf[3] { // Address type
	case 0x01: // IPv4
		if n < 10 {
			return fmt.Errorf("invalid IPv4 address length")
		}
		targetAddr = fmt.Sprintf("%d.%d.%d.%d", buf[4], buf[5], buf[6], buf[7])
		targetPort = uint16(buf[8])<<8 | uint16(buf[9])
	case 0x03: // Domain name
		if n < 5 {
			return fmt.Errorf("invalid domain name length")
		}
		domainLen := int(buf[4])
		if n < 5+domainLen+2 {
			return fmt.Errorf("incomplete domain name")
		}
		targetAddr = string(buf[5 : 5+domainLen])
		targetPort = uint16(buf[5+domainLen])<<8 | uint16(buf[5+domainLen+1])
	case 0x04: // IPv6
		if n < 22 {
			return fmt.Errorf("invalid IPv6 address length")
		}
		// IPv6 address parsing
		ipv6 := net.IP(buf[4:20])
		targetAddr = ipv6.String()
		targetPort = uint16(buf[20])<<8 | uint16(buf[21])
	default:
		return fmt.Errorf("unsupported address type: %d", buf[3])
	}

	target := fmt.Sprintf("%s:%d", targetAddr, targetPort)

	// Connect to target through SSH tunnel
	remoteConn, err := s.sshConn.Dial("tcp", target)
	if err != nil {
		// Send connection failed response
		response := []byte{0x05, 0x05, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		clientConn.Write(response)
		return fmt.Errorf("failed to connect to target %s: %v", target, err)
	}
	defer remoteConn.Close()

	// Send success response
	response := []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err = clientConn.Write(response)
	if err != nil {
		return fmt.Errorf("failed to send success response: %v", err)
	}

	if commonConfig.Debug {
		log.Printf("SOCKS5 connection established to %s", target)
	}

	// Start bidirectional data transfer and wait for completion
	done := make(chan bool, 2)

	go func() {
		copyConn(clientConn, remoteConn, commonConfig)
		done <- true
	}()

	go func() {
		copyConn(remoteConn, clientConn, commonConfig)
		done <- true
	}()

	// Wait for either direction to complete
	<-done

	return nil
}

func (s *socks5Server) handleUsernamePasswordAuth(clientConn net.Conn, commonConfig *CommonConfig) error {
	buf := make([]byte, 256)
	n, err := clientConn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read auth request: %v", err)
	}

	if n < 2 || buf[0] != 0x01 {
		return fmt.Errorf("invalid auth version")
	}

	// Parse username
	userLen := int(buf[1])
	if n < 2+userLen+1 {
		return fmt.Errorf("invalid username length")
	}
	username := string(buf[2 : 2+userLen])

	// Parse password
	passLen := int(buf[2+userLen])
	if n < 2+userLen+1+passLen {
		return fmt.Errorf("invalid password length")
	}
	password := string(buf[2+userLen+1 : 2+userLen+1+passLen])

	// Verify credentials
	if username == s.config.Socks5User && password == s.config.Socks5Pass {
		// Authentication successful
		_, err = clientConn.Write([]byte{0x01, 0x00})
		if err != nil {
			return fmt.Errorf("failed to send auth success: %v", err)
		}
		if commonConfig.Debug {
			log.Printf("SOCKS5 authentication successful for user: %s", username)
		}
		return nil
	} else {
		// Authentication failed
		_, err = clientConn.Write([]byte{0x01, 0x01})
		if err != nil {
			return fmt.Errorf("failed to send auth failure: %v", err)
		}
		return fmt.Errorf("invalid credentials for user: %s", username)
	}
}

func handleReverseSocks5Proxy(conn *ssh.Client, config *ForwardConfig, commonConfig *CommonConfig) error {
	// Listen on remote server
	listener, err := conn.Listen("tcp", fmt.Sprintf("%s:%s", config.RemoteIP, config.RemotePort))
	if err != nil {
		return fmt.Errorf("failed to listen on remote server: %v", err)
	}
	defer listener.Close()

	log.Printf("Reverse SOCKS5 proxy listening on remote %s:%s", config.RemoteIP, config.RemotePort)

	for {
		remoteConn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection: %v", err)
		}

		go handleReverseSocks5Connection(remoteConn, config, commonConfig)
	}
}

func handleReverseSocks5Connection(remoteConn net.Conn, config *ForwardConfig, commonConfig *CommonConfig) {
	defer remoteConn.Close()

	// Create a reverse SOCKS5 server that dials to local network
	reverseSocks5Server := &reverseSocks5Server{config: config}

	// Handle the SOCKS5 protocol
	err := reverseSocks5Server.handleConnection(remoteConn, commonConfig)
	if err != nil {
		log.Printf("Reverse SOCKS5 connection error: %v", err)
	}
}

type reverseSocks5Server struct {
	config *ForwardConfig
}

func (s *reverseSocks5Server) handleConnection(clientConn net.Conn, commonConfig *CommonConfig) error {
	// Read SOCKS5 version and number of authentication methods
	buf := make([]byte, 256)
	n, err := clientConn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read SOCKS5 greeting: %v", err)
	}

	if n < 2 || buf[0] != 0x05 {
		return fmt.Errorf("invalid SOCKS5 version")
	}

	// Check if authentication is required
	requireAuth := s.config.Socks5User != "" && s.config.Socks5Pass != ""

	// Parse supported authentication methods
	numMethods := int(buf[1])
	if n < 2+numMethods {
		return fmt.Errorf("invalid authentication methods")
	}

	supportedMethods := buf[2 : 2+numMethods]
	var selectedMethod byte = 0xFF // No acceptable methods

	if requireAuth {
		// Check if client supports username/password authentication (method 0x02)
		for _, method := range supportedMethods {
			if method == 0x02 {
				selectedMethod = 0x02
				break
			}
		}
	} else {
		// Check if client supports no authentication (method 0x00)
		for _, method := range supportedMethods {
			if method == 0x00 {
				selectedMethod = 0x00
				break
			}
		}
	}

	// Send authentication method selection response
	_, err = clientConn.Write([]byte{0x05, selectedMethod})
	if err != nil {
		return fmt.Errorf("failed to send auth method response: %v", err)
	}

	if selectedMethod == 0xFF {
		return fmt.Errorf("no acceptable authentication methods")
	}

	// Handle authentication if required
	if selectedMethod == 0x02 {
		err = s.handleUsernamePasswordAuth(clientConn, commonConfig)
		if err != nil {
			return fmt.Errorf("authentication failed: %v", err)
		}
	}

	// Read connection request
	n, err = clientConn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read connection request: %v", err)
	}

	if n < 4 || buf[0] != 0x05 || buf[1] != 0x01 {
		return fmt.Errorf("invalid SOCKS5 connection request")
	}

	// Parse target address
	var targetAddr string
	var targetPort uint16

	switch buf[3] { // Address type
	case 0x01: // IPv4
		if n < 10 {
			return fmt.Errorf("invalid IPv4 address length")
		}
		targetAddr = fmt.Sprintf("%d.%d.%d.%d", buf[4], buf[5], buf[6], buf[7])
		targetPort = uint16(buf[8])<<8 | uint16(buf[9])
	case 0x03: // Domain name
		if n < 5 {
			return fmt.Errorf("invalid domain name length")
		}
		domainLen := int(buf[4])
		if n < 5+domainLen+2 {
			return fmt.Errorf("incomplete domain name")
		}
		targetAddr = string(buf[5 : 5+domainLen])
		targetPort = uint16(buf[5+domainLen])<<8 | uint16(buf[5+domainLen+1])
	case 0x04: // IPv6
		if n < 22 {
			return fmt.Errorf("invalid IPv6 address length")
		}
		// IPv6 address parsing
		ipv6 := net.IP(buf[4:20])
		targetAddr = ipv6.String()
		targetPort = uint16(buf[20])<<8 | uint16(buf[21])
	default:
		return fmt.Errorf("unsupported address type: %d", buf[3])
	}

	target := fmt.Sprintf("%s:%d", targetAddr, targetPort)

	// Add DNS resolution debugging for domain names
	if buf[3] == 0x03 { // Domain name
		_, err := net.LookupIP(targetAddr)
		if err != nil {
			log.Printf("Reverse SOCKS5 DNS resolution failed for %s: %v", targetAddr, err)
		}
	}

	// For reverse SOCKS5, we need to connect through the local machine's internet connection
	// This allows the remote server to access the internet through our local connection
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}
	localConn, err := dialer.Dial("tcp", target)
	if err != nil {
		if commonConfig.Debug {
			log.Printf("Reverse SOCKS5 connection failed to %s: %v", target, err)
		}
		// Send connection failed response
		response := []byte{0x05, 0x05, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		clientConn.Write(response)
		return fmt.Errorf("failed to connect to target %s through local connection: %v", target, err)
	}
	defer localConn.Close()

	// Send success response
	response := []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err = clientConn.Write(response)
	if err != nil {
		return fmt.Errorf("failed to send success response: %v", err)
	}

	if commonConfig.Debug {
		log.Printf("Reverse SOCKS5 connection established: %s", target)
	}

	// Start bidirectional data transfer and wait for completion
	done := make(chan bool, 2)

	go func() {
		copyConn(clientConn, localConn, commonConfig)
		done <- true
	}()

	go func() {
		copyConn(localConn, clientConn, commonConfig)
		done <- true
	}()

	// Wait for either direction to complete
	<-done

	return nil
}

func (s *reverseSocks5Server) handleUsernamePasswordAuth(clientConn net.Conn, commonConfig *CommonConfig) error {
	buf := make([]byte, 256)
	n, err := clientConn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read auth request: %v", err)
	}

	if n < 2 || buf[0] != 0x01 {
		return fmt.Errorf("invalid auth version")
	}

	// Parse username
	userLen := int(buf[1])
	if n < 2+userLen+1 {
		return fmt.Errorf("invalid username length")
	}
	username := string(buf[2 : 2+userLen])

	// Parse password
	passLen := int(buf[2+userLen])
	if n < 2+userLen+1+passLen {
		return fmt.Errorf("invalid password length")
	}
	password := string(buf[2+userLen+1 : 2+userLen+1+passLen])

	// Verify credentials
	if username == s.config.Socks5User && password == s.config.Socks5Pass {
		// Authentication successful
		_, err = clientConn.Write([]byte{0x01, 0x00})
		if err != nil {
			return fmt.Errorf("failed to send auth success: %v", err)
		}
		if commonConfig.Debug {
			log.Printf("Reverse SOCKS5 authentication successful for user: %s", username)
		}
		return nil
	} else {
		// Authentication failed
		_, err = clientConn.Write([]byte{0x01, 0x01})
		if err != nil {
			return fmt.Errorf("failed to send auth failure: %v", err)
		}
		return fmt.Errorf("invalid credentials for user: %s", username)
	}
}

func copyConn(dst io.WriteCloser, src io.ReadCloser, commonConfig *CommonConfig) {
	defer dst.Close()
	defer src.Close()

	_, err := io.Copy(dst, src)
	if err != nil && err != io.EOF && commonConfig.Debug {
		log.Printf("Data transfer error: %v", err)
	}
}

// Connection manager methods
func (cm *ConnectionManager) GetConnection(serverName string) (*ssh.Client, error) {
	cm.mutex.RLock()
	if conn, exists := cm.connections[serverName]; exists && conn != nil {
		cm.mutex.RUnlock()
		return conn, nil
	}
	cm.mutex.RUnlock()

	// Connection doesn't exist, create it
	return cm.createConnection(serverName)
}

func (cm *ConnectionManager) createConnection(serverName string) (*ssh.Client, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Double-check after acquiring write lock
	if conn, exists := cm.connections[serverName]; exists && conn != nil {
		return conn, nil
	}

	// Get server config
	serverConfig, ok := servers[serverName]
	if !ok {
		return nil, fmt.Errorf("server configuration not found for %s", serverName)
	}

	// Create SSH config
	sshConfig := &ssh.ClientConfig{
		User: serverConfig.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(serverConfig.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Establish connection
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", serverConfig.Server, serverConfig.Port), sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %v", serverName, err)
	}

	// Store connection
	cm.connections[serverName] = conn

	// Start connection monitor
	go cm.monitorConnection(serverName, conn)

	log.Printf("Created shared SSH connection for server: %s", serverName)
	return conn, nil
}

func (cm *ConnectionManager) monitorConnection(serverName string, conn *ssh.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if connection is still alive
			if conn.Conn == nil {
				log.Printf("SSH connection lost for server: %s", serverName)
				goto cleanup
			}
			// Send a keep-alive ping
			_, _, err := conn.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				log.Printf("SSH connection failed for server: %s: %v", serverName, err)
				goto cleanup
			}
		case <-cm.ctx.Done():
			log.Printf("Context cancelled, closing SSH connection for server: %s", serverName)
			goto cleanup
		}
	}

cleanup:
	// Remove connection from map
	cm.mutex.Lock()
	delete(cm.connections, serverName)
	cm.mutex.Unlock()
}

func (cm *ConnectionManager) CloseAll() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	for serverName, conn := range cm.connections {
		if conn != nil {
			conn.Close()
			log.Printf("Closed SSH connection for server: %s", serverName)
		}
	}
	cm.connections = make(map[string]*ssh.Client)
}

func (cm *ConnectionManager) RemoveConnection(serverName string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if conn, exists := cm.connections[serverName]; exists && conn != nil {
		conn.Close()
		log.Printf("Removed failed SSH connection for server: %s", serverName)
	}
	delete(cm.connections, serverName)
}
