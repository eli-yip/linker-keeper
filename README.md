# LinkerBot Keeper

[中文文档](./README_zhCN.md)

A lightweight, web-based process management tool written in Go that provides monitoring, control, and automatic restart capabilities for system processes.

## Features

- 🚀 **Process Management**: Start, stop, restart processes with ease
- 🔄 **Auto-restart**: Configurable automatic restart on process failure
- 🌐 **Web Interface**: Clean, responsive web UI for process monitoring
- 📊 **Real-time Monitoring**: Live process status, PID tracking, and output logs
- ⚙️ **Flexible Configuration**: Support for JSON and YAML configuration files
- 🔐 **User Management**: Run processes as different users (with sudo support)
- 📝 **Logging**: Capture and display process stdout/stderr
- 🔧 **Hot Reload**: Dynamic configuration reloading without restart

## Quick Start

### Installation

1. **Download or compile the binary:**
   ```bash
   # Clone the repository
   git clone https://github.com/soulteary/linkerbot-keeper.git
   cd linkerbot-keeper
   
   # Build from source
   go build -o keeper main.go
   ```

2. **Run with default configuration:**
   ```bash
   ./keeper
   ```

3. **Access the web interface:**
   Open your browser and navigate to `http://localhost:8080`

## Configuration

### Configuration File

LinkerBot Keeper supports both YAML and JSON configuration formats. By default, it looks for `keeper.yaml` in the current directory.

#### YAML Configuration Example

```yaml
server:
  port: "8080"
  host: "0.0.0.0"
  refresh_time: 10

processes:
  - name: "web-server"
    command: "/usr/bin/nginx"
    args: ["-g", "daemon off;"]
    workdir: "/etc/nginx"
    auto_restart: true
    enabled: true
    environment:
      ENV: "production"
      PORT: "80"
    user: "www-data"
    max_restarts: 5
    restart_delay: 10
    description: "Nginx web server"

  - name: "api-service"
    command: "/opt/myapp/api-server"
    args: ["--config", "/opt/myapp/config.json"]
    workdir: "/opt/myapp"
    auto_restart: true
    enabled: true
    environment:
      DATABASE_URL: "postgres://localhost/mydb"
      LOG_LEVEL: "info"
    max_restarts: 10
    restart_delay: 5
    description: "REST API service"
```

#### JSON Configuration Example

```json
{
  "server": {
    "port": "8080",
    "host": "0.0.0.0",
    "refresh_time": 10
  },
  "processes": [
    {
      "name": "web-server",
      "command": "/usr/bin/nginx",
      "args": ["-g", "daemon off;"],
      "workdir": "/etc/nginx",
      "auto_restart": true,
      "enabled": true,
      "environment": {
        "ENV": "production",
        "PORT": "80"
      },
      "user": "www-data",
      "max_restarts": 5,
      "restart_delay": 10,
      "description": "Nginx web server"
    }
  ]
}
```

### Configuration Options

#### Server Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `port` | string | "8080" | Web interface port |
| `host` | string | "0.0.0.0" | Web interface host |
| `refresh_time` | int | 10 | Auto-refresh interval (seconds) |

#### Process Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | ✅ | Unique process identifier |
| `command` | string | ✅ | Executable path or command |
| `args` | []string | ❌ | Command line arguments |
| `workdir` | string | ❌ | Working directory (default: ".") |
| `auto_restart` | bool | ❌ | Enable automatic restart on failure |
| `enabled` | bool | ❌ | Whether process should start automatically |
| `environment` | map[string]string | ❌ | Environment variables |
| `user` | string | ❌ | Run process as specific user (requires sudo) |
| `max_restarts` | int | ❌ | Maximum restart attempts (default: 10) |
| `restart_delay` | int | ❌ | Delay between restarts in seconds (default: 5) |
| `description` | string | ❌ | Human-readable process description |

## Usage

### Command Line

```bash
# Run with default config file (keeper.yaml)
./keeper

# Run with custom config file
./keeper /path/to/config.yaml

# Run with JSON config
./keeper /path/to/config.json
```

### Web Interface

The web interface provides:

- **Process Overview**: Real-time status of all configured processes
- **Process Controls**: Start, stop, restart buttons for each process
- **Log Viewing**: Click "日志" (Logs) to view process output
- **Configuration Reload**: Reload config without restarting the manager
- **Auto-refresh**: Configurable automatic page refresh

### API Endpoints

LinkerBot Keeper provides REST API endpoints for programmatic control:

#### Process Control
- `POST /api/process/{name}/start` - Start a process
- `POST /api/process/{name}/stop` - Stop a process  
- `POST /api/process/{name}/restart` - Restart a process

#### Management
- `POST /api/enable/{name}` - Enable auto-restart for a process
- `POST /api/reload` - Reload configuration
- `GET /api/status` - Get all process statuses
- `GET /api/logs/{name}` - Get process logs
- `GET /api/config` - Get current configuration

#### Example API Usage

```bash
# Start a process
curl -X POST http://localhost:8080/api/process/web-server/start

# Get process status
curl http://localhost:8080/api/status

# View logs
curl http://localhost:8080/api/logs/web-server

# Reload configuration
curl -X POST http://localhost:8080/api/reload
```

## Advanced Features

### Automatic Restart Logic

- Processes are automatically restarted when they exit unexpectedly
- Restart counter prevents infinite restart loops
- When `max_restarts` is reached, auto-restart is disabled
- Use "启用重启" (Enable Restart) button to reset counter and re-enable

### User Management

LinkerBot Keeper can run processes as different users:

```yaml
processes:
  - name: "secure-service"
    command: "/opt/secure/service"
    user: "serviceuser"  # Will use sudo to run as this user
    # ...
```

### Environment Variables

Set custom environment variables for each process:

```yaml
processes:
  - name: "app"
    command: "/opt/app/server"
    environment:
      DATABASE_URL: "postgres://localhost/app"
      REDIS_URL: "redis://localhost:6379"
      LOG_LEVEL: "debug"
```

### Working Directory

Specify the working directory for each process:

```yaml
processes:
  - name: "web-app"
    command: "./start.sh"
    workdir: "/opt/webapp"
```

## Deployment

### Systemd Service

```bash
curl -L https://raw.githubusercontent.com/linker-bot/linker-keeper/refs/heads/main/scripts/install.sh | sudo bash
```

## Security Considerations

1. **Sudo Access**: When running processes as different users, ensure the keeper process has appropriate sudo permissions
2. **File Permissions**: Secure your configuration files with appropriate permissions
3. **Network Access**: Consider restricting web interface access using firewalls or reverse proxies
4. **Process Security**: Validate that managed processes have appropriate security configurations

## Troubleshooting

### Common Issues

**Process won't start:**
- Check if the executable exists and has proper permissions
- Verify the working directory exists
- Check environment variables and user permissions

**Web interface not accessible:**
- Verify the port is not in use by another service
- Check firewall settings
- Ensure the host/port configuration is correct

**Auto-restart not working:**
- Check if `auto_restart` is enabled in configuration
- Verify the process hasn't exceeded `max_restarts`
- Look at process logs for error messages

### Log Analysis

Use the web interface or API to check process logs:
- Process stdout/stderr are captured automatically
- Logs show timestamps and stream types (STDOUT/STDERR)
- Last 50 log lines are retained per process

## Contributing

We welcome contributions! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Submit a pull request

### Development Setup

```bash
# Clone the repository
git clone https://github.com/soulteary/linkerbot-keeper.git
cd linkerbot-keeper

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o keeper main.go
```

## License

This project is licensed under the Apache License - see the [LICENSE](LICENSE) file for details.

---

**LinkerBot Keeper** - Simple, reliable process management for modern applications.