package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/crypto/ssh"
	"gopkg.in/ini.v1"

	"image/color"
)

type ServerConfig struct {
	Server   string
	User     string
	Password string
}

type ForwardConfig struct {
	Name       string
	ServerName string
	RemoteIP   string
	RemotePort string
	LocalIP    string
	LocalPort  string
	Direction  string
	SSHConfig  *ServerConfig
	Status     bool
	StatusItem *canvas.Circle
}

var (
	forwardConfigs []*ForwardConfig
	servers        map[string]*ServerConfig
	configMutex    sync.Mutex
)

func main() {
	servers = make(map[string]*ServerConfig)
	loadConfig()

	myApp := app.New()
	myWindow := myApp.NewWindow("SSH Port Forwarder")

	var list *widget.List
	list = widget.NewList(
		func() int {
			return len(forwardConfigs)
		},
		func() fyne.CanvasObject {
			circle := canvas.NewCircle(color.Gray{Y: 0x99})
			circle.Resize(fyne.NewSize(10, 10))
			return container.NewHBox(
				widget.NewButton("Placeholder", nil),
				circle,
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			config := forwardConfigs[id]
			items := item.(*fyne.Container).Objects
			button := items[0].(*widget.Button)
			button.SetText(fmt.Sprintf("%s %s: %s:%s -> %s:%s", config.Name, config.ServerName, config.LocalIP, config.LocalPort, config.RemoteIP, config.RemotePort))

			button.OnTapped = func() {
				editPortForwardDialog(myWindow, list, config)
			}

			statusCircle := items[1].(*canvas.Circle)
			// set the size of the circle to 10x10
			statusCircle.Resize(fyne.NewSize(10, 10))
			if config.Status {
				statusCircle.FillColor = color.RGBA{R: 0x00, G: 0xff, B: 0x00, A: 0xff}
			} else {
				statusCircle.FillColor = color.RGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
			}
			statusCircle.Refresh()
			config.StatusItem = statusCircle
		},
	)

	addButton := widget.NewButton("Add Port Forward", func() {
		addPortForwardDialog(myWindow, list)
	})

	addServerButton := widget.NewButton("Add Server", func() {
		addServerDialog(myWindow)
	})

	buttons := container.NewHBox(addButton, addServerButton)
	content := container.NewBorder(nil, buttons, nil, nil, list)
	myWindow.SetContent(content)

	go startForwarders()

	myWindow.Resize(fyne.NewSize(400, 400))
	myWindow.ShowAndRun()
	//disallow resizing
	myWindow.SetFixedSize(true)
}

func addServerDialog(window fyne.Window) {
	serverNameEntry := widget.NewEntry()
	serverNameEntry.SetPlaceHolder("Server Name")

	serverAddressEntry := widget.NewEntry()
	serverAddressEntry.SetPlaceHolder("Server Address")

	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("Username")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	var dialog *widget.PopUp
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Server Name", Widget: serverNameEntry},
			{Text: "Server Address", Widget: serverAddressEntry},
			{Text: "Username", Widget: userEntry},
			{Text: "Password", Widget: passwordEntry},
		},
		OnSubmit: func() {
			newServer := &ServerConfig{
				Server:   serverAddressEntry.Text,
				User:     userEntry.Text,
				Password: passwordEntry.Text,
			}

			configMutex.Lock()
			servers[serverNameEntry.Text] = newServer
			configMutex.Unlock()

			saveConfig()
		},
		OnCancel: func() {
			dialog.Hide()
		},
	}

	dialog = widget.NewModalPopUp(form, window.Canvas())
	dialog.Resize(fyne.NewSize(300, 300))
	dialog.Show()
}

func addPortForwardDialog(window fyne.Window, list *widget.List) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Forward Name")

	serverEntry := widget.NewEntry()
	serverEntry.SetPlaceHolder("Server")

	remoteIPEntry := widget.NewEntry()
	remoteIPEntry.SetPlaceHolder("Remote IP")

	remotePortEntry := widget.NewEntry()
	remotePortEntry.SetPlaceHolder("Remote Port")

	localIPEntry := widget.NewEntry()
	localIPEntry.SetPlaceHolder("Local IP")

	localPortEntry := widget.NewEntry()
	localPortEntry.SetPlaceHolder("Local Port")

	directionSelect := widget.NewSelect([]string{"local", "remote"}, nil)
	directionSelect.SetSelected("local")

	var dialog *widget.PopUp
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Name", Widget: nameEntry},
			{Text: "Server", Widget: serverEntry},
			{Text: "Remote IP", Widget: remoteIPEntry},
			{Text: "Remote Port", Widget: remotePortEntry},
			{Text: "Local IP", Widget: localIPEntry},
			{Text: "Local Port", Widget: localPortEntry},
			{Text: "Direction", Widget: directionSelect},
		},
		OnSubmit: func() {
			newConfig := &ForwardConfig{
				Name:       nameEntry.Text,
				ServerName: serverEntry.Text,
				RemoteIP:   remoteIPEntry.Text,
				RemotePort: remotePortEntry.Text,
				LocalIP:    localIPEntry.Text,
				LocalPort:  localPortEntry.Text,
				Direction:  directionSelect.Selected,
				SSHConfig:  servers[serverEntry.Text],
			}

			configMutex.Lock()
			forwardConfigs = append(forwardConfigs, newConfig)
			configMutex.Unlock()

			list.Refresh()
			go handleConnection(newConfig)
			saveConfig()
		},
		OnCancel: func() {
			dialog.Hide()
		},
	}

	dialog = widget.NewModalPopUp(form, window.Canvas())
	dialog.Resize(fyne.NewSize(300, 400))
	dialog.Show()
}

func editPortForwardDialog(window fyne.Window, list *widget.List, config *ForwardConfig) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(config.Name)

	serverEntry := widget.NewEntry()
	serverEntry.SetText(config.ServerName)

	remoteIPEntry := widget.NewEntry()
	remoteIPEntry.SetText(config.RemoteIP)

	remotePortEntry := widget.NewEntry()
	remotePortEntry.SetText(config.RemotePort)

	localIPEntry := widget.NewEntry()
	localIPEntry.SetText(config.LocalIP)

	localPortEntry := widget.NewEntry()
	localPortEntry.SetText(config.LocalPort)

	directionSelect := widget.NewSelect([]string{"local", "remote"}, nil)
	directionSelect.SetSelected(config.Direction)

	var dialog *widget.PopUp
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Name", Widget: nameEntry},
			{Text: "Server", Widget: serverEntry},
			{Text: "Remote IP", Widget: remoteIPEntry},
			{Text: "Remote Port", Widget: remotePortEntry},
			{Text: "Local IP", Widget: localIPEntry},
			{Text: "Local Port", Widget: localPortEntry},
			{Text: "Direction", Widget: directionSelect},
		},
		OnSubmit: func() {
			config.Name = nameEntry.Text
			config.ServerName = serverEntry.Text
			config.RemoteIP = remoteIPEntry.Text
			config.RemotePort = remotePortEntry.Text
			config.LocalIP = localIPEntry.Text
			config.LocalPort = localPortEntry.Text
			config.Direction = directionSelect.Selected
			config.SSHConfig = servers[serverEntry.Text]

			list.Refresh()
			saveConfig()
			dialog.Hide()
		},
		OnCancel: func() {
			dialog.Hide()
		},
	}

	dialog = widget.NewModalPopUp(form, window.Canvas())
	dialog.Resize(fyne.NewSize(300, 400))
	dialog.Show()
}

func loadConfig() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Printf("Failed to load config file: %v", err)
		return
	}

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
				Name:       section.Name(),
				ServerName: section.Key("server").String(),
				RemoteIP:   section.Key("remoteIP").String(),
				RemotePort: section.Key("remotePort").String(),
				LocalIP:    section.Key("localIP").String(),
				LocalPort:  section.Key("localPort").String(),
				Direction:  section.Key("direction").String(),
			}
			if sshConfig, ok := servers[forwardConfig.ServerName]; ok {
				forwardConfig.SSHConfig = sshConfig
			}
			forwardConfigs = append(forwardConfigs, forwardConfig)
		}
	}
}

func startForwarders() {
	for _, config := range forwardConfigs {
		go handleConnection(config)
	}
}

func handleConnection(config *ForwardConfig) {
	for {
		err := connectAndForward(config)
		if err != nil {
			log.Printf("Error in connection for %s: %v. Retrying in 30 seconds...", config.ServerName, err)
			config.Status = false
			if config.StatusItem != nil {
				config.StatusItem.FillColor = color.RGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
				config.StatusItem.Refresh()
			}
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
	config.Status = true
	if config.StatusItem != nil {
		config.StatusItem.FillColor = color.RGBA{R: 0x00, G: 0xff, B: 0x00, A: 0xff}
		config.StatusItem.Refresh()
	}

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

func saveConfig() {
	cfg := ini.Empty()

	for serverName, serverConfig := range servers {
		section, _ := cfg.NewSection(serverName)
		section.NewKey("server", serverConfig.Server)
		section.NewKey("user", serverConfig.User)
		section.NewKey("password", serverConfig.Password)
	}

	for _, forwardConfig := range forwardConfigs {
		section, _ := cfg.NewSection(forwardConfig.Name)
		section.NewKey("server", forwardConfig.ServerName)
		section.NewKey("remoteIP", forwardConfig.RemoteIP)
		section.NewKey("remotePort", forwardConfig.RemotePort)
		section.NewKey("localIP", forwardConfig.LocalIP)
		section.NewKey("localPort", forwardConfig.LocalPort)
		section.NewKey("direction", forwardConfig.Direction)
	}

	err := cfg.SaveTo("config.ini")
	if err != nil {
		log.Printf("Failed to save config file: %v", err)
	}
}
