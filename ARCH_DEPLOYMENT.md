# ReAI Deployment on Arch Linux (Raspberry Pi)

This guide will help you deploy ReAI natively on Arch Linux ARM for Raspberry Pi, avoiding Docker compatibility issues.

## Prerequisites

- Raspberry Pi with Arch Linux ARM installed
- SSH access to your Pi
- Internet connection

## Option 1: Automated Setup (Recommended)

1. SSH into your Raspberry Pi:
   ```bash
   ssh your-username@your-pi-ip
   ```

2. Download and run the setup script:
   ```bash
   curl -sSL https://raw.githubusercontent.com/itsalfredakku/ReAI/main/setup-arch.sh | bash
   ```

## Option 2: Manual Setup

### 1. Install Dependencies

```bash
# Update system
sudo pacman -Syu

# Install Go and required packages
sudo pacman -S --needed go git wget curl base-devel
```

### 2. Clone and Build

```bash
# Clone the repository
git clone https://github.com/itsalfredakku/ReAI.git
cd ReAI

# Build the application
go mod tidy
go build -o bin/reai ./cmd/server
```

### 3. Create Systemd Service

```bash
# Create service file
sudo tee /etc/systemd/system/reai.service > /dev/null <<EOF
[Unit]
Description=ReAI - OpenAI Compatible API Server
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=$(pwd)
ExecStart=$(pwd)/bin/reai
Restart=always
RestartSec=5

Environment=PORT=8080
Environment=DATA_DIR=$(pwd)/data
Environment=LOG_LEVEL=info

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$(pwd)/data

[Install]
WantedBy=multi-user.target
EOF

# Create data directory
mkdir -p data

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable reai
sudo systemctl start reai
```

### 4. Configure Firewall (Optional)

```bash
# If using ufw
sudo ufw allow 8080/tcp
sudo ufw reload
```

## Service Management

```bash
# Check status
sudo systemctl status reai

# Start service
sudo systemctl start reai

# Stop service
sudo systemctl stop reai

# Restart service
sudo systemctl restart reai

# View logs
sudo journalctl -u reai -f

# View recent logs
sudo journalctl -u reai --since "1 hour ago"
```

## Testing the API

```bash
# Health check
curl http://localhost:8080/health

# List models
curl http://localhost:8080/v1/models

# Test completion (replace with your actual prompt)
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "def fibonacci(n):",
    "language": "python",
    "max_tokens": 100
  }'
```

## Configuration

Environment variables can be set in the systemd service file or in a `.env` file:

```bash
# Edit service file
sudo systemctl edit reai

# Add environment variables
[Service]
Environment=PORT=8080
Environment=DATA_DIR=/path/to/data
Environment=LOG_LEVEL=debug
Environment=COPILOT_CLIENT_ID=your_client_id
```

## Remote Access

To access the API from other devices on your network:

1. Update the service to bind to all interfaces:
   ```bash
   # Edit the service file to include
   Environment=HOST=0.0.0.0
   ```

2. Configure firewall:
   ```bash
   sudo ufw allow from 192.168.1.0/24 to any port 8080
   ```

3. Find your Pi's IP address:
   ```bash
   ip addr show
   ```

4. Access from other devices:
   ```bash
   curl http://YOUR_PI_IP:8080/health
   ```

## Troubleshooting

### Service won't start
```bash
# Check service status
sudo systemctl status reai

# Check logs
sudo journalctl -u reai --since "10 minutes ago"

# Check if port is in use
sudo netstat -tulpn | grep :8080
```

### Build issues
```bash
# Verify Go installation
go version

# Clean and rebuild
go clean -cache
go mod tidy
go build -o bin/reai ./cmd/server
```

### Permission issues
```bash
# Fix ownership
sudo chown -R $(whoami):$(whoami) /path/to/ReAI

# Fix permissions
chmod +x bin/reai
chmod -R 755 data/
```

## Updating ReAI

```bash
cd ReAI
git pull origin main
go build -o bin/reai ./cmd/server
sudo systemctl restart reai
```

## Uninstall

```bash
# Stop and disable service
sudo systemctl stop reai
sudo systemctl disable reai

# Remove service file
sudo rm /etc/systemd/system/reai.service
sudo systemctl daemon-reload

# Remove application
rm -rf ReAI/
```

## Performance Tips

1. **Disable swap** on Pi for better performance:
   ```bash
   sudo dphys-swapfile swapoff
   sudo dphys-swapfile uninstall
   sudo systemctl disable dphys-swapfile
   ```

2. **Overclock** (if using adequate cooling):
   ```bash
   # Edit /boot/config.txt
   sudo nano /boot/config.txt
   
   # Add:
   arm_freq=1750
   gpu_freq=500
   ```

3. **Use SSD** instead of SD card for better I/O performance.

## Support

If you encounter issues:
1. Check the logs: `sudo journalctl -u reai -f`
2. Verify network connectivity
3. Ensure all dependencies are installed
4. Check GitHub issues: https://github.com/itsalfredakku/ReAI/issues
