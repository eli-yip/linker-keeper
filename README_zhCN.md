# LinkerBot Keeper

一个用 Go 编写的轻量级、基于 Web 的进程管理工具，为系统进程提供监控、控制和自动重启功能。

## 特性

- 🚀 **进程管理**：轻松启动、停止、重启进程
- 🔄 **自动重启**：可配置的进程失败自动重启
- 🌐 **Web 界面**：简洁、响应式的进程监控 Web UI
- 📊 **实时监控**：实时进程状态、PID 跟踪和输出日志
- ⚙️ **灵活配置**：支持 JSON 和 YAML 配置文件格式
- 🔐 **用户管理**：以不同用户身份运行进程（支持 sudo）
- 📝 **日志记录**：捕获并显示进程 stdout/stderr
- 🔧 **热重载**：无需重启即可动态重新加载配置

## 快速开始

### 安装

1. **下载或编译二进制文件：**
   ```bash
   # 克隆仓库
   git clone https://github.com/soulteary/linkerbot-keeper.git
   cd linkerbot-keeper
   
   # 从源码构建
   go build -o keeper main.go
   ```

2. **使用默认配置运行：**
   ```bash
   ./keeper
   ```

3. **访问 Web 界面：**
   打开浏览器并访问 `http://localhost:8080`

## 配置

### 配置文件

LinkerBot Keeper 支持 YAML 和 JSON 配置文件格式。默认情况下，它会在当前目录下查找 `keeper.yaml`。

#### YAML 配置示例

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
    description: "Nginx web 服务器"

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
    description: "REST API 服务"
```

#### JSON 配置示例

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
      "description": "Nginx web 服务器"
    }
  ]
}
```

### 配置选项

#### 服务器配置

| 字段 | 类型 | 默认值 | 描述 |
|-------|------|---------|-------------|
| `port` | string | "8080" | Web 界面端口 |
| `host` | string | "0.0.0.0" | Web 界面主机 |
| `refresh_time` | int | 10 | 自动刷新间隔（秒） |

#### 进程配置

| 字段 | 类型 | 必需 | 描述 |
|-------|------|----------|-------------|
| `name` | string | ✅ | 唯一进程标识符 |
| `command` | string | ✅ | 可执行文件路径或命令 |
| `args` | []string | ❌ | 命令行参数 |
| `workdir` | string | ❌ | 工作目录（默认："."） |
| `auto_restart` | bool | ❌ | 启用失败时自动重启 |
| `enabled` | bool | ❌ | 进程是否应自动启动 |
| `environment` | map[string]string | ❌ | 环境变量 |
| `user` | string | ❌ | 以特定用户身份运行进程（需要 sudo） |
| `max_restarts` | int | ❌ | 最大重启次数（默认：10） |
| `restart_delay` | int | ❌ | 重启间隔秒数（默认：5） |
| `description` | string | ❌ | 进程的可读描述 |

## 使用方法

### 命令行

```bash
# 使用默认配置文件运行 (keeper.yaml)
./keeper

# 使用自定义配置文件运行
./keeper /path/to/config.yaml

# 使用 JSON 配置运行
./keeper /path/to/config.json
```

### Web 界面

Web 界面提供：

- **进程概览**：所有配置进程的实时状态
- **进程控制**：每个进程的启动、停止、重启按钮
- **日志查看**：点击"日志"查看进程输出
- **配置重载**：无需重启管理器即可重新加载配置
- **自动刷新**：可配置的自动页面刷新

### API 端点

LinkerBot Keeper 提供 REST API 端点用于程序化控制：

#### 进程控制
- `POST /api/process/{name}/start` - 启动进程
- `POST /api/process/{name}/stop` - 停止进程
- `POST /api/process/{name}/restart` - 重启进程

#### 管理
- `POST /api/enable/{name}` - 为进程启用自动重启
- `POST /api/reload` - 重新加载配置
- `GET /api/status` - 获取所有进程状态
- `GET /api/logs/{name}` - 获取进程日志
- `GET /api/config` - 获取当前配置

#### API 使用示例

```bash
# 启动进程
curl -X POST http://localhost:8080/api/process/web-server/start

# 获取进程状态
curl http://localhost:8080/api/status

# 查看日志
curl http://localhost:8080/api/logs/web-server

# 重新加载配置
curl -X POST http://localhost:8080/api/reload
```

## 高级功能

### 自动重启逻辑

- 进程在意外退出时会自动重启
- 重启计数器防止无限重启循环
- 当达到 `max_restarts` 时，自动重启被禁用
- 使用"启用重启"按钮重置计数器并重新启用

### 用户管理

LinkerBot Keeper 可以以不同用户身份运行进程：

```yaml
processes:
  - name: "secure-service"
    command: "/opt/secure/service"
    user: "serviceuser"  # 将使用 sudo 以此用户身份运行
    # ...
```

### 环境变量

为每个进程设置自定义环境变量：

```yaml
processes:
  - name: "app"
    command: "/opt/app/server"
    environment:
      DATABASE_URL: "postgres://localhost/app"
      REDIS_URL: "redis://localhost:6379"
      LOG_LEVEL: "debug"
```

### 工作目录

为每个进程指定工作目录：

```yaml
processes:
  - name: "web-app"
    command: "./start.sh"
    workdir: "/opt/webapp"
```

## 部署

### Systemd 服务

为 LinkerBot Keeper 创建 systemd 服务文件：

```ini
[Unit]
Description=LinkerBot Keeper 进程管理器
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/keeper /etc/keeper/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# 安装并启动服务
sudo cp keeper.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable keeper
sudo systemctl start keeper
```

### Docker 部署

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o keeper main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates sudo
WORKDIR /root/
COPY --from=builder /app/keeper .
COPY config.yaml .
EXPOSE 8080
CMD ["./keeper"]
```

```bash
# 构建并运行
docker build -t linkerbot-keeper .
docker run -d -p 8080:8080 -v /path/to/config.yaml:/root/config.yaml linkerbot-keeper
```

## 安全考虑

1. **Sudo 访问**：当以不同用户身份运行进程时，确保 keeper 进程具有适当的 sudo 权限
2. **文件权限**：使用适当的权限保护配置文件
3. **网络访问**：考虑使用防火墙或反向代理限制 Web 界面访问
4. **进程安全**：验证受管理的进程具有适当的安全配置

## 故障排除

### 常见问题

**进程无法启动：**
- 检查可执行文件是否存在并具有适当权限
- 验证工作目录是否存在
- 检查环境变量和用户权限

**Web 界面无法访问：**
- 验证端口是否被其他服务占用
- 检查防火墙设置
- 确保主机/端口配置正确

**自动重启不工作：**
- 检查配置中是否启用了 `auto_restart`
- 验证进程是否未超过 `max_restarts`
- 查看进程日志中的错误消息

### 日志分析

使用 Web 界面或 API 检查进程日志：
- 进程 stdout/stderr 会自动捕获
- 日志显示时间戳和流类型（STDOUT/STDERR）
- 每个进程保留最近 50 行日志

## 贡献

我们欢迎贡献！请遵循以下指南：

1. Fork 仓库
2. 创建功能分支
3. 进行更改并添加测试
4. 提交 pull request

### 开发设置

```bash
# 克隆仓库
git clone https://github.com/soulteary/linkerbot-keeper.git
cd linkerbot-keeper

# 安装依赖
go mod download

# 运行测试
go test ./...

# 构建
go build -o keeper main.go
```

## 许可证

本项目采用 Apache 许可证 - 详见 [LICENSE](LICENSE) 文件。

---

**LinkerBot Keeper** - 现代应用程序的简单、可靠的进程管理。