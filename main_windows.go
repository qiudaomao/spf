//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"golang.org/x/crypto/ssh"
	"gopkg.in/ini.v1"
)

// Struct definitions
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
	SectionName string // Original section name from config.ini
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

// SOCKS5 server types
type socks5Server struct {
	sshConn *ssh.Client
	config  *ForwardConfig
}

type reverseSocks5Server struct {
	config *ForwardConfig
}

var (
	cfg            *ini.File
	commonConfig   *CommonConfig
	servers        map[string]*ServerConfig
	forwardConfigs []*ForwardConfig
	connManager    *ConnectionManager
	ctx            context.Context
	cancel         context.CancelFunc
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

	// Load configuration
	var err error
	cfg, err = ini.Load("config.ini")
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	// Parse common configuration
	commonConfig = &CommonConfig{}
	if cfg.HasSection("common") {
		commonSection := cfg.Section("common")
		commonConfig.Debug = commonSection.Key("debug").MustBool(false)
	}

	// Parse server configurations
	servers = make(map[string]*ServerConfig)
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

	// Link forward configs to server configs
	for _, fc := range forwardConfigs {
		if sshConfig, ok := servers[fc.ServerName]; ok {
			fc.SSHConfig = sshConfig
		} else {
			log.Printf("Warning: No server configuration found for %s", fc.ServerName)
		}
	}

	// Start the system tray
	systray.Run(onReady, onExit)
}

func onReady() {
	// Set icon
	iconPath := "icon.ico"
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		log.Fatalf("Icon file not found: %s. Please provide an icon.ico file.", iconPath)
	}
	systray.SetIcon(getIcon(iconPath))
	systray.SetTitle("SSH Port Forwarder")
	systray.SetTooltip("SSH Port Forwarder - Running")

	// Add menu items
	systray.AddMenuItem("Status: Running", "Status")
	systray.AddSeparator()

	// Group forward configurations by server
	serverGroups := make(map[string][]*ForwardConfig)
	for _, fc := range forwardConfigs {
		if fc.SSHConfig != nil {
			serverGroups[fc.ServerName] = append(serverGroups[fc.ServerName], fc)
		}
	}

	// Create menu structure grouped by server
	for serverName, configs := range serverGroups {
		// Add server section header with connection status
		serverMenuItem := systray.AddMenuItem(fmt.Sprintf("%s", serverName), fmt.Sprintf("Server: %s", serverName))
		serverMenuItem.Disable() // Make it non-clickable

		// Add port configurations under this server
		for _, fc := range configs {
			var name string
			var tooltip string

			switch fc.Direction {
			case "remote":
				name = fmt.Sprintf("  %s %s:%s r → l %s:%s", fc.SectionName, fc.RemoteIP, fc.RemotePort, fc.LocalIP, fc.LocalPort)
				tooltip = fmt.Sprintf("Remote port forward: %s:%s → %s:%s", fc.RemoteIP, fc.RemotePort, fc.LocalIP, fc.LocalPort)
			case "local":
				name = fmt.Sprintf("  %s %s:%s l → r %s:%s", fc.SectionName, fc.LocalIP, fc.LocalPort, fc.RemoteIP, fc.RemotePort)
				tooltip = fmt.Sprintf("Local port forward: %s:%s ← %s:%s", fc.LocalIP, fc.LocalPort, fc.RemoteIP, fc.RemotePort)
			case "socks5":
				name = fmt.Sprintf("  %s %s:%s l ← SOCKS5", fc.SectionName, fc.LocalIP, fc.LocalPort)
				tooltip = fmt.Sprintf("SOCKS5 proxy: %s:%s", fc.LocalIP, fc.LocalPort)
			case "reverse-socks5":
				name = fmt.Sprintf("  %s %s:%s r → SOCKS5", fc.SectionName, fc.RemoteIP, fc.RemotePort)
				tooltip = fmt.Sprintf("Reverse SOCKS5 proxy: %s:%s", fc.RemoteIP, fc.RemotePort)
			default:
				name = fmt.Sprintf("  %s (Unknown)", fc.SectionName)
				tooltip = fmt.Sprintf("Unknown direction: %s", fc.Direction)
			}

			menuItem := systray.AddMenuItem(name, tooltip)
			go handleMenuItemClick(menuItem, fc)
		}

		// Add separator between servers
		systray.AddSeparator()
	}

	/*
		systray.AddSeparator()
		showLogMenuItem := systray.AddMenuItem("Show Log", "Show Log")
		reloadConfigMenuItem := systray.AddMenuItem("Reload Config", "Reload Config")
		go handleShowLogMenuItemClick(showLogMenuItem)
		go handleReloadConfigMenuItemClick(reloadConfigMenuItem)
	*/
	quitMenuItem := systray.AddMenuItem("Quit", "Quit")
	go handleQuitMenuItemClick(quitMenuItem)

	// Start all forward connections
	for _, fc := range forwardConfigs {
		if fc.SSHConfig != nil {
			go handleConnection(fc, commonConfig)
		}
	}
}

func onExit() {
	// Cancel all running operations
	cancel()

	// Close all shared SSH connections
	if connManager != nil {
		connManager.CloseAll()
	}

	log.Println("Shutting down SSH Port Forwarder...")
}

func handleMenuItemClick(menuItem *systray.MenuItem, config *ForwardConfig) {
	for range menuItem.ClickedCh {
		// Show detailed status for this configuration
		log.Printf("=== Configuration Details ===")
		log.Printf("Section: %s", config.SectionName)
		log.Printf("Server: %s (%s:%s)", config.ServerName, config.SSHConfig.Server, config.SSHConfig.Port)
		log.Printf("Direction: %s", config.Direction)

		switch config.Direction {
		case "remote":
			log.Printf("Remote Port Forward: %s:%s → %s:%s",
				config.RemoteIP, config.RemotePort, config.LocalIP, config.LocalPort)
		case "local":
			log.Printf("Local Port Forward: %s:%s ← %s:%s",
				config.LocalIP, config.LocalPort, config.RemoteIP, config.RemotePort)
		case "socks5":
			log.Printf("SOCKS5 Proxy: %s:%s", config.LocalIP, config.LocalPort)
			if config.Socks5User != "" {
				log.Printf("SOCKS5 Auth: %s", config.Socks5User)
			}
		case "reverse-socks5":
			log.Printf("Reverse SOCKS5 Proxy: %s:%s", config.RemoteIP, config.RemotePort)
			if config.Socks5User != "" {
				log.Printf("SOCKS5 Auth: %s", config.Socks5User)
			}
		}
		log.Printf("================================")
	}
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
		select {
		case <-ctx.Done():
			return nil
		default:
			remoteConn, err := listener.Accept()
			if err != nil {
				return fmt.Errorf("failed to accept connection: %v", err)
			}

			go handleForwardingConnection(remoteConn, config.LocalIP, config.LocalPort, commonConfig)
		}
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
		select {
		case <-ctx.Done():
			return nil
		default:
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
		select {
		case <-ctx.Done():
			return nil
		default:
			clientConn, err := listener.Accept()
			if err != nil {
				return fmt.Errorf("failed to accept connection: %v", err)
			}

			go handleSocks5Connection(clientConn, conn, config, commonConfig)
		}
	}
}

func handleSocks5Connection(clientConn net.Conn, sshConn *ssh.Client, config *ForwardConfig, commonConfig *CommonConfig) {
	defer clientConn.Close()

	socks5Server := &socks5Server{
		sshConn: sshConn,
		config:  config,
	}

	err := socks5Server.handleConnection(clientConn, commonConfig)
	if err != nil {
		log.Printf("SOCKS5 connection error: %v", err)
	}
}

func handleReverseSocks5Proxy(conn *ssh.Client, config *ForwardConfig, commonConfig *CommonConfig) error {
	listener, err := conn.Listen("tcp", fmt.Sprintf("%s:%s", config.RemoteIP, config.RemotePort))
	if err != nil {
		return fmt.Errorf("failed to listen on remote server: %v", err)
	}
	defer listener.Close()

	log.Printf("Reverse SOCKS5 proxy listening on remote %s:%s", config.RemoteIP, config.RemotePort)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			remoteConn, err := listener.Accept()
			if err != nil {
				return fmt.Errorf("failed to accept connection: %v", err)
			}

			go handleReverseSocks5Connection(remoteConn, config, commonConfig)
		}
	}
}

func handleReverseSocks5Connection(remoteConn net.Conn, config *ForwardConfig, commonConfig *CommonConfig) {
	defer remoteConn.Close()

	reverseSocks5Server := &reverseSocks5Server{config: config}

	err := reverseSocks5Server.handleConnection(remoteConn, commonConfig)
	if err != nil {
		log.Printf("Reverse SOCKS5 connection error: %v", err)
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

// Helper functions for icon handling
func getIcon(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Failed to read icon file: %v", err)
		return nil
	}
	return data
}

// SOCKS5 server method implementations
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

func handleQuitMenuItemClick(menuItem *systray.MenuItem) {
	for range menuItem.ClickedCh {
		log.Println("Quitting SSH Port Forwarder...")
		systray.Quit()
	}
}

func handleShowLogMenuItemClick(menuItem *systray.MenuItem) {
	for range menuItem.ClickedCh {
		// Implementation of Show Log menu item click handler
		log.Println("Show Log menu item clicked")
	}
}

func handleReloadConfigMenuItemClick(menuItem *systray.MenuItem) {
	for range menuItem.ClickedCh {
		// Implementation of Reload Config menu item click handler
		log.Println("Reload Config menu item clicked")
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

// Helper functions for connection status
func getConnectionStatus(serverName string) bool {
	if connManager == nil {
		return false
	}

	connManager.mutex.RLock()
	defer connManager.mutex.RUnlock()

	if conn, exists := connManager.connections[serverName]; exists && conn != nil {
		// Check if connection is still alive
		if conn.Conn != nil {
			// Try to send a keep-alive ping
			_, _, err := conn.SendRequest("keepalive@openssh.com", true, nil)
			return err == nil
		}
	}
	return false
}

func getStatusText(connected bool) string {
	if connected {
		return "Connected"
	}
	return "Disconnected"
}
