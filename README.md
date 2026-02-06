# mDNS Reflector
A simple mDNS reflector written in Go.

## Installation
### From Binary (Linux)
Download the latest binary from the [Releases](../../releases) page:

```bash
# AMD64
curl -sSL https://github.com/sugtao4423/mDNS-Reflector/releases/latest/download/mdns-reflector-linux-amd64 -o mdns-reflector
# ARM64
curl -sSL https://github.com/sugtao4423/mDNS-Reflector/releases/latest/download/mdns-reflector-linux-arm64 -o mdns-reflector

chmod +x mdns-reflector
sudo mv mdns-reflector /usr/local/bin/
```

### Build from Source
```bash
git clone https://github.com/sugtao4423/mDNS-Reflector.git
cd mDNS-Reflector
go build -o mdns-reflector
```

## Usage
### List Available Interfaces
```bash
./mdns-reflector -l
```

### Basic Usage
Reflect mDNS between two interfaces:

```bash
./mdns-reflector -i eth0,wlan0
```

Supports three or more interfaces:

```bash
./mdns-reflector -i eth0,eth1,docker0
```

### Debug Mode
Monitor packet forwarding:

```bash
./mdns-reflector -i eth0,wlan0 -d
```

## Options
Option | Description
--- | ---
`-i` | Comma-separated list of interface names (required)
`-d` | Enable debug logging
`-l` | List available network interfaces
`-v` | Show version information

## Running as systemd Service
```text
[Unit]
Description=mDNS Reflector
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/mdns-reflector -i eth0,wlan0
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo cp mdns-reflector /usr/local/bin/
sudo cp mdns-reflector.service /etc/systemd/system/

sudo systemctl daemon-reload
sudo systemctl enable mdns-reflector
sudo systemctl start mdns-reflector
```
