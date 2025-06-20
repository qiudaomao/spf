GOOS=windows GOARCH=amd64 go build -o ./release/spf.exe main.go
GOOS=linux GOARCH=amd64 go build -o ./release/spf_linux_amd64 main.go
go build -o ./release/spf_darwin main.go
