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
	ServerName  string
	RemoteIP    string
	RemotePort  string
	LocalIP     string
	LocalPort   string
	Direction   string
	SSHConfig   *ServerConfig
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

func copyConn(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}