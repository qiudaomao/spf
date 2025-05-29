package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/ini.v1"
)

type ServerConfig struct {
	Server   string
	User     string
	Password string
}

type ForwardConfig struct {
	ServerName string
	RemoteIP   string
	RemotePort string
	LocalIP    string
	LocalPort  string
	Direction  string
	SSHConfig  *ServerConfig
	// SOCKS5 authentication
	Socks5User string
	Socks5Pass string
}

func main() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	servers := make(map[string]*ServerConfig)
	var forwardConfigs []*ForwardConfig

	for _, section := range cfg.Sections() {
		if section.Name() == "DEFAULT" {
			continue
		}

		if section.HasKey("user") && section.HasKey("password") {
			servers[section.Name()] = &ServerConfig{
				Server:   section.Key("server").String(),
				User:     section.Key("user").String(),
				Password: section.Key("password").String(),
			}
		} else if section.HasKey("server") && section.HasKey("direction") {
			forwardConfig := &ForwardConfig{
				ServerName: section.Key("server").String(),
				RemoteIP:   section.Key("remoteIP").String(),
				RemotePort: section.Key("remotePort").String(),
				LocalIP:    section.Key("localIP").String(),
				LocalPort:  section.Key("localPort").String(),
				Direction:  section.Key("direction").String(),
				Socks5User: section.Key("socks5User").String(),
				Socks5Pass: section.Key("socks5Pass").String(),
			}
			forwardConfigs = append(forwardConfigs, forwardConfig)
		}
	}

	for _, fc := range forwardConfigs {
		if sshConfig, ok := servers[fc.ServerName]; ok {
			fc.SSHConfig = sshConfig
			go handleConnection(fc)
		} else {
			log.Printf("Warning: No server configuration found for %s", fc.ServerName)
		}
	}

	// Keep the main goroutine running
	select {}
}

func handleConnection(config *ForwardConfig) {
	for {
		err := connectAndForward(config)
		if err != nil {
			log.Printf("Error in connection for %s: %v. Retrying in 30 seconds...", config.ServerName, err)
			time.Sleep(30 * time.Second)
		}
	}
}

func connectAndForward(config *ForwardConfig) error {
	sshConfig := &ssh.ClientConfig{
		User: config.SSHConfig.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(config.SSHConfig.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", config.SSHConfig.Server), sshConfig)
	if err != nil {
		return fmt.Errorf("failed to dial: %v", err)
	}
	defer conn.Close()

	log.Printf("Connected to %s", config.SSHConfig.Server)

	switch config.Direction {
	case "remote":
		err = handleRemotePortForward(conn, config)
	case "local":
		err = handleLocalPortForward(conn, config)
	case "socks5":
		err = handleSocks5Proxy(conn, config)
	case "reverse-socks5":
		err = handleReverseSocks5Proxy(conn, config)
	default:
		return fmt.Errorf("invalid direction: %s", config.Direction)
	}

	return err
}

func handleRemotePortForward(conn *ssh.Client, config *ForwardConfig) error {
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

		go handleForwardingConnection(remoteConn, config.LocalIP, config.LocalPort)
	}
}

func handleLocalPortForward(conn *ssh.Client, config *ForwardConfig) error {
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

			go copyConn(localConn, remoteConn)
			go copyConn(remoteConn, localConn)
		}()
	}
}

func handleForwardingConnection(incomingConn net.Conn, targetIP, targetPort string) {
	targetConn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", targetIP, targetPort))
	if err != nil {
		log.Printf("Failed to connect to target address: %v", err)
		incomingConn.Close()
		return
	}

	go copyConn(targetConn, incomingConn)
	go copyConn(incomingConn, targetConn)
}

func handleSocks5Proxy(conn *ssh.Client, config *ForwardConfig) error {
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

		go handleSocks5Connection(clientConn, conn, config)
	}
}

func handleSocks5Connection(clientConn net.Conn, sshConn *ssh.Client, config *ForwardConfig) {
	defer clientConn.Close()

	// Create a SOCKS5 server that uses the SSH connection for dialing
	socks5Server := &socks5Server{
		sshConn: sshConn,
		config:  config,
	}

	// Handle the SOCKS5 protocol
	err := socks5Server.handleConnection(clientConn)
	if err != nil {
		log.Printf("SOCKS5 connection error: %v", err)
	}
}

type socks5Server struct {
	sshConn *ssh.Client
	config  *ForwardConfig
}

func (s *socks5Server) handleConnection(clientConn net.Conn) error {
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
		err = s.handleUsernamePasswordAuth(clientConn)
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

	log.Printf("SOCKS5 connection established to %s", target)

	// Start bidirectional data transfer
	go copyConn(clientConn, remoteConn)
	go copyConn(remoteConn, clientConn)

	return nil
}

func (s *socks5Server) handleUsernamePasswordAuth(clientConn net.Conn) error {
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
		log.Printf("SOCKS5 authentication successful for user: %s", username)
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

func handleReverseSocks5Proxy(conn *ssh.Client, config *ForwardConfig) error {
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

		go handleReverseSocks5Connection(remoteConn, config)
	}
}

func handleReverseSocks5Connection(remoteConn net.Conn, config *ForwardConfig) {
	defer remoteConn.Close()

	// Create a reverse SOCKS5 server that dials to local network
	reverseSocks5Server := &reverseSocks5Server{config: config}

	// Handle the SOCKS5 protocol
	err := reverseSocks5Server.handleConnection(remoteConn)
	if err != nil {
		log.Printf("Reverse SOCKS5 connection error: %v", err)
	}
}

type reverseSocks5Server struct {
	config *ForwardConfig
}

func (s *reverseSocks5Server) handleConnection(clientConn net.Conn) error {
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
		err = s.handleUsernamePasswordAuth(clientConn)
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

	// Connect to target through local network (direct connection)
	localConn, err := net.Dial("tcp", target)
	if err != nil {
		// Send connection failed response
		response := []byte{0x05, 0x05, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		clientConn.Write(response)
		return fmt.Errorf("failed to connect to local target %s: %v", target, err)
	}
	defer localConn.Close()

	// Send success response
	response := []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err = clientConn.Write(response)
	if err != nil {
		return fmt.Errorf("failed to send success response: %v", err)
	}

	log.Printf("Reverse SOCKS5 connection established to local %s", target)

	// Start bidirectional data transfer
	go copyConn(clientConn, localConn)
	go copyConn(localConn, clientConn)

	return nil
}

func (s *reverseSocks5Server) handleUsernamePasswordAuth(clientConn net.Conn) error {
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
		log.Printf("Reverse SOCKS5 authentication successful for user: %s", username)
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

func copyConn(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}
