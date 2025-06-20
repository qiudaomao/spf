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
	"time"

	"github.com/getlantern/systray"
	"golang.org/x/crypto/ssh"
	"gopkg.in/ini.v1"
)

var (
	cfg            *ini.File
	commonConfig   *CommonConfig
	servers        map[string]*ServerConfig
	forwardConfigs []*ForwardConfig
	ctx            context.Context
	cancel         context.CancelFunc
)

func main() {
	// Initialize context for graceful shutdown
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

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
	iconPath := "icon.png"
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		// Create a default icon if icon.png doesn't exist
		createDefaultIcon(iconPath)
	}
	systray.SetIcon(getIcon(iconPath))
	systray.SetTitle("SSH Port Forwarder")
	systray.SetTooltip("SSH Port Forwarder - Running")

	// Add menu items
	systray.AddMenuItem("Status: Running", "Status")
	systray.AddSeparator()

	// Add forward configurations to menu
	for _, fc := range forwardConfigs {
		if fc.SSHConfig != nil {
			menuItem := systray.AddMenuItem(
				fmt.Sprintf("%s (%s)", fc.ServerName, fc.Direction),
				fmt.Sprintf("Configuration for %s", fc.ServerName),
			)
			go handleMenuItemClick(menuItem, fc)
		}
	}

	systray.AddSeparator()
	systray.AddMenuItem("Show Log", "Show Log")
	systray.AddMenuItem("Reload Config", "Reload Config")
	systray.AddSeparator()
	systray.AddMenuItem("Quit", "Quit")

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
	log.Println("Shutting down SSH Port Forwarder...")
}

func handleMenuItemClick(menuItem *systray.MenuItem, config *ForwardConfig) {
	for range menuItem.ClickedCh {
		// Show status for this configuration
		log.Printf("Configuration: %s (%s) - %s:%s -> %s:%s",
			config.ServerName, config.Direction,
			config.LocalIP, config.LocalPort,
			config.RemoteIP, config.RemotePort)
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
				log.Printf("Error in connection for %s: %v. Retrying in 30 seconds...", config.ServerName, err)
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
	sshConfig := &ssh.ClientConfig{
		User: config.SSHConfig.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(config.SSHConfig.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", config.SSHConfig.Server, config.SSHConfig.Port), sshConfig)
	if err != nil {
		return fmt.Errorf("failed to dial: %v", err)
	}
	defer conn.Close()

	log.Printf("Connected to %s", config.SSHConfig.Server)

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

func createDefaultIcon(path string) {
	// Create a simple default icon if icon.png doesn't exist
	// This is a minimal 16x16 PNG icon data
	defaultIcon := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0xf3, 0xff, 0x61, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x60, 0x18, 0x05, 0x03,
		0x00, 0x00, 0x30, 0x00, 0x00, 0x01, 0x57, 0x6d, 0xb7, 0x4a, 0x00, 0x00,
		0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}

	err := os.WriteFile(path, defaultIcon, 0644)
	if err != nil {
		log.Printf("Failed to create default icon: %v", err)
	}
}
