run spf

with a config.ini in same directory

```ini
[serverA]
server=192.168.111.92
user=root
password=abc123

[ssh]
server=serverA
remoteIP=127.0.0.1
remotePort=22
localIP=127.0.0.1
localPort=8922
direction=local

[rdp]
server=serverA
remoteIP=0.0.0.0
remotePort=2289
localIP=127.0.0.1
localPort=3389
direction=remote
```
