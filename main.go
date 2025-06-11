package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

// ProcessConfig 进程配置
type ProcessConfig struct {
	Name         string            `json:"name" yaml:"name"`
	Command      string            `json:"command" yaml:"command"`
	Args         []string          `json:"args" yaml:"args"`
	WorkDir      string            `json:"workdir" yaml:"workdir"`
	AutoRestart  bool              `json:"auto_restart" yaml:"auto_restart"`
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	Environment  map[string]string `json:"environment" yaml:"environment"`
	User         string            `json:"user" yaml:"user"`
	MaxRestarts  int               `json:"max_restarts" yaml:"max_restarts"`
	RestartDelay int               `json:"restart_delay" yaml:"restart_delay"` // 重启延迟秒数
	Description  string            `json:"description" yaml:"description"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port        string `json:"port" yaml:"port"`
	Host        string `json:"host" yaml:"host"`
	RefreshTime int    `json:"refresh_time" yaml:"refresh_time"` // 页面刷新时间
}

// Config 总配置
type Config struct {
	Server    ServerConfig    `json:"server" yaml:"server"`
	Processes []ProcessConfig `json:"processes" yaml:"processes"`
}

// ProcessStatus 进程状态
type ProcessStatus struct {
	Config       ProcessConfig `json:"config"`
	PID          int           `json:"pid"`
	Status       string        `json:"status"` // running, stopped, error, disabled
	StartTime    time.Time     `json:"start_time"`
	Restarts     int           `json:"restarts"`
	LastError    string        `json:"last_error"`
	LastExitCode int           `json:"last_exit_code"`
	Output       []string      `json:"output"` // 最近的输出日志
}

// ProcessInfo 进程运行信息
type ProcessInfo struct {
	Cmd     *exec.Cmd
	Cancel  context.CancelFunc
	Context context.Context
}

// ProcessManager 进程管理器
type ProcessManager struct {
	processes    map[string]*ProcessStatus
	commands     map[string]*ProcessInfo
	mutex        sync.RWMutex
	config       *Config
	configPath   string
	lastModified time.Time
}

// NewProcessManager 创建新的进程管理器
func NewProcessManager(configPath string) *ProcessManager {
	return &ProcessManager{
		processes:  make(map[string]*ProcessStatus),
		commands:   make(map[string]*ProcessInfo),
		configPath: configPath,
	}
}

// getDefaultConfig 获取默认配置
func getDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:        "8080",
			Host:        "0.0.0.0",
			RefreshTime: 10,
		},
		Processes: []ProcessConfig{
			{
				Name:         "example-service",
				Command:      "/bin/echo",
				Args:         []string{"Hello World"},
				WorkDir:      "/tmp",
				AutoRestart:  true,
				Enabled:      false,
				Environment:  map[string]string{"ENV": "production"},
				User:         "",
				MaxRestarts:  10,
				RestartDelay: 5,
				Description:  "示例服务 - 请修改配置文件",
			},
		},
	}
}

// LoadConfig 加载配置
func (pm *ProcessManager) LoadConfig() error {
	// 检查配置文件是否存在
	if _, err := os.Stat(pm.configPath); os.IsNotExist(err) {
		log.Printf("配置文件 %s 不存在，创建默认配置", pm.configPath)
		return pm.createDefaultConfig()
	}

	// 检查文件是否被修改
	fileInfo, err := os.Stat(pm.configPath)
	if err != nil {
		return fmt.Errorf("无法获取配置文件信息: %v", err)
	}

	// 如果文件未被修改，且已加载过配置，则跳过
	if !fileInfo.ModTime().After(pm.lastModified) && pm.config != nil {
		return nil
	}

	// 读取配置文件
	data, err := os.ReadFile(pm.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	ext := strings.ToLower(filepath.Ext(pm.configPath))

	switch ext {
	case ".json":
		err = json.Unmarshal(data, &config)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &config)
	default:
		return fmt.Errorf("不支持的配置文件格式: %s，支持 .json, .yaml, .yml", ext)
	}

	if err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 验证配置
	if err := pm.validateConfig(&config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.config = &config
	pm.lastModified = fileInfo.ModTime()

	// 更新进程配置
	for _, processConfig := range config.Processes {
		if existing, exists := pm.processes[processConfig.Name]; exists {
			// 更新现有进程配置
			existing.Config = processConfig
		} else {
			// 添加新进程
			pm.processes[processConfig.Name] = &ProcessStatus{
				Config: processConfig,
				Status: "stopped",
				Output: make([]string, 0, 50),
			}
		}
	}

	log.Printf("配置加载成功，管理 %d 个进程", len(config.Processes))
	return nil
}

// createDefaultConfig 创建默认配置文件
func (pm *ProcessManager) createDefaultConfig() error {
	config := getDefaultConfig()
	pm.config = config

	var data []byte
	var err error

	ext := strings.ToLower(filepath.Ext(pm.configPath))
	switch ext {
	case ".json":
		data, err = json.MarshalIndent(config, "", "  ")
	case ".yaml", ".yml":
		data, err = yaml.Marshal(config)
	default:
		// 默认使用 YAML 格式
		pm.configPath = pm.configPath + ".yaml"
		data, err = yaml.Marshal(config)
	}

	if err != nil {
		return fmt.Errorf("序列化默认配置失败: %v", err)
	}

	// 确保目录存在
	dir := filepath.Dir(pm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	err = os.WriteFile(pm.configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("写入默认配置文件失败: %v", err)
	}

	log.Printf("已创建默认配置文件: %s", pm.configPath)

	// 初始化进程状态
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	for _, processConfig := range config.Processes {
		pm.processes[processConfig.Name] = &ProcessStatus{
			Config: processConfig,
			Status: "stopped",
			Output: make([]string, 0, 50),
		}
	}

	return nil
}

// validateConfig 验证配置
func (pm *ProcessManager) validateConfig(config *Config) error {
	// 验证服务器配置
	if config.Server.Port == "" {
		config.Server.Port = "8080"
	}
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if config.Server.RefreshTime <= 0 {
		config.Server.RefreshTime = 10
	}

	// 验证进程配置
	processNames := make(map[string]bool)
	for i, processConfig := range config.Processes {
		if processConfig.Name == "" {
			return fmt.Errorf("进程 [%d] 名称不能为空", i)
		}
		if processNames[processConfig.Name] {
			return fmt.Errorf("进程名称重复: %s", processConfig.Name)
		}
		processNames[processConfig.Name] = true

		if processConfig.Command == "" {
			return fmt.Errorf("进程[%s]命令不能为空", processConfig.Name)
		}

		// 设置默认值
		if processConfig.MaxRestarts <= 0 {
			config.Processes[i].MaxRestarts = 10
		}
		if processConfig.RestartDelay <= 0 {
			config.Processes[i].RestartDelay = 5
		}
		if processConfig.WorkDir == "" {
			config.Processes[i].WorkDir = "."
		}
	}

	return nil
}

// StartProcess 启动进程
func (pm *ProcessManager) StartProcess(name string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	status, exists := pm.processes[name]
	if !exists {
		return fmt.Errorf("进程 %s 不存在", name)
	}

	if status.Status == "running" {
		return fmt.Errorf("进程 %s 已经在运行", name)
	}

	if !status.Config.Enabled {
		return fmt.Errorf("进程 %s 已被禁用", name)
	}

	config := status.Config

	// 检查可执行文件是否存在
	execPath := config.Command
	if !filepath.IsAbs(execPath) {
		// 如果不是绝对路径，在 PATH 中查找
		if _, err := exec.LookPath(execPath); err != nil {
			status.Status = "error"
			status.LastError = fmt.Sprintf("命令不存在: %s", execPath)
			pm.addLog(name, fmt.Sprintf("ERROR: 命令不存在: %s", execPath))
			return fmt.Errorf("命令不存在: %s", execPath)
		}
	} else {
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			status.Status = "error"
			status.LastError = fmt.Sprintf("可执行文件不存在: %s", execPath)
			pm.addLog(name, fmt.Sprintf("ERROR: 可执行文件不存在: %s", execPath))
			return fmt.Errorf("可执行文件不存在: %s", execPath)
		}
	}

	// 检查重启次数限制
	if status.Restarts >= config.MaxRestarts {
		status.Status = "disabled"
		status.Config.AutoRestart = false
		pm.addLog(name, fmt.Sprintf("ERROR: 重启次数过多 (%d次)，已禁用自动重启", status.Restarts))
		return fmt.Errorf("进程 %s 重启次数过多，已禁用", name)
	}

	// 创建上下文用于进程控制
	ctx, cancel := context.WithCancel(context.Background())

	// 构建命令
	var cmd *exec.Cmd
	if needsSudo(config.Command, config.User) {
		// 使用 sudo 启动
		args := buildSudoArgs(config)
		cmd = exec.CommandContext(ctx, "sudo", args...)
	} else {
		// 过滤掉空参数
		var filteredArgs []string
		for _, arg := range config.Args {
			if arg != "" {
				filteredArgs = append(filteredArgs, arg)
			}
		}
		cmd = exec.CommandContext(ctx, config.Command, filteredArgs...)
	}

	// 设置工作目录
	if config.WorkDir != "" {
		cmd.Dir = config.WorkDir
	}

	// 设置环境变量
	if len(config.Environment) > 0 {
		env := os.Environ()
		for key, value := range config.Environment {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		cmd.Env = env
	}

	// 设置进程组，便于管理子进程
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	// 捕获输出
	cmd.Stdout = &logWriter{name: name, pm: pm, isStdout: true}
	cmd.Stderr = &logWriter{name: name, pm: pm, isStdout: false}

	// 启动进程
	err := cmd.Start()
	if err != nil {
		cancel()
		status.Status = "error"
		status.LastError = err.Error()
		pm.addLog(name, fmt.Sprintf("ERROR: 启动失败: %v", err))
		return fmt.Errorf("启动进程 %s 失败: %v", name, err)
	}

	// 保存进程信息
	pm.commands[name] = &ProcessInfo{
		Cmd:     cmd,
		Cancel:  cancel,
		Context: ctx,
	}

	status.PID = cmd.Process.Pid
	status.Status = "running"
	status.StartTime = time.Now()
	status.LastError = ""

	pm.addLog(name, fmt.Sprintf("INFO: 进程启动成功，PID: %d", status.PID))

	// 监控进程状态
	go pm.monitorProcess(name)

	log.Printf("进程 %s 启动成功，PID: %d", name, status.PID)
	return nil
}

// buildSudoArgs 构建 sudo 命令参数
func buildSudoArgs(config ProcessConfig) []string {
	args := []string{}

	// 如果指定了用户，添加-u 参数
	if config.User != "" {
		args = append(args, "-u", config.User)
	}

	args = append(args, config.Command)

	// 添加进程参数
	for _, arg := range config.Args {
		if arg != "" {
			args = append(args, arg)
		}
	}

	return args
}

// StopProcess 停止进程
func (pm *ProcessManager) StopProcess(name string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	status, exists := pm.processes[name]
	if !exists {
		return fmt.Errorf("进程 %s 不存在", name)
	}

	procInfo, cmdExists := pm.commands[name]
	if !cmdExists || status.Status != "running" {
		return fmt.Errorf("进程 %s 没有运行", name)
	}

	pm.addLog(name, "INFO: 正在停止进程...")

	// 取消上下文
	procInfo.Cancel()

	// 给进程一些时间优雅退出
	done := make(chan error, 1)
	go func() {
		done <- procInfo.Cmd.Wait()
	}()

	// 等待 5 秒，如果还没退出就强制杀死
	select {
	case <-done:
		// 进程已经退出
	case <-time.After(5 * time.Second):
		// 超时，强制杀死进程组
		if procInfo.Cmd.Process != nil {
			syscall.Kill(-procInfo.Cmd.Process.Pid, syscall.SIGKILL)
			<-done // 等待 Wait() 完成
		}
		pm.addLog(name, "WARNING: 进程未在 5 秒内退出，已强制终止")
	}

	delete(pm.commands, name)

	status.Status = "stopped"
	status.PID = 0

	pm.addLog(name, "INFO: 进程已手动停止")
	log.Printf("进程 %s 已停止", name)
	return nil
}

// RestartProcess 重启进程
func (pm *ProcessManager) RestartProcess(name string) error {
	// 先停止进程
	err := pm.StopProcess(name)
	if err != nil && !strings.Contains(err.Error(), "没有运行") {
		return err
	}

	// 等待指定时间后重启
	pm.mutex.RLock()
	delay := 2
	if status, exists := pm.processes[name]; exists {
		delay = status.Config.RestartDelay
	}
	pm.mutex.RUnlock()

	time.Sleep(time.Duration(delay) * time.Second)
	return pm.StartProcess(name)
}

// EnableAutoRestart 启用自动重启
func (pm *ProcessManager) EnableAutoRestart(name string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	status, exists := pm.processes[name]
	if !exists {
		return fmt.Errorf("进程 %s 不存在", name)
	}

	status.Config.AutoRestart = true
	status.Config.Enabled = true
	status.Restarts = 0 // 重置重启计数
	if status.Status == "disabled" {
		status.Status = "stopped"
	}

	pm.addLog(name, "INFO: 已启用自动重启并重置重启计数")
	return nil
}

// monitorProcess 监控进程状态
func (pm *ProcessManager) monitorProcess(name string) {
	pm.mutex.RLock()
	procInfo, exists := pm.commands[name]
	if !exists {
		pm.mutex.RUnlock()
		return
	}
	cmd := procInfo.Cmd
	pm.mutex.RUnlock()

	err := cmd.Wait()

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	status := pm.processes[name]
	delete(pm.commands, name)

	// 获取退出状态码
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}

		// 如果是被取消的上下文，说明是正常停止
		if err == context.Canceled {
			pm.addLog(name, "INFO: 进程正常停止")
			log.Printf("进程 %s 正常停止", name)
		} else {
			status.LastError = err.Error()
			pm.addLog(name, fmt.Sprintf("ERROR: 进程异常退出: %v (退出码: %d)", err, exitCode))
			log.Printf("进程 %s 异常退出: %v (退出码: %d)", name, err, exitCode)
		}
	} else {
		pm.addLog(name, "INFO: 进程正常退出")
		log.Printf("进程 %s 正常退出", name)
	}

	status.Status = "stopped"
	status.PID = 0
	status.LastExitCode = exitCode

	// 只有在异常退出时才增加重启计数
	if err != nil && err != context.Canceled {
		status.Restarts++

		// 如果重启次数过多，禁用自动重启
		if status.Restarts >= status.Config.MaxRestarts {
			log.Printf("进程 %s 重启次数过多(%d次)，禁用自动重启", name, status.Restarts)
			status.Config.AutoRestart = false
			status.Status = "disabled"
			pm.addLog(name, fmt.Sprintf("WARNING: 重启次数过多 (%d次)，已禁用自动重启", status.Restarts))
			return
		}

		// 自动重启
		if status.Config.AutoRestart && status.Config.Enabled {
			restartDelay := status.Config.RestartDelay
			pm.addLog(name, fmt.Sprintf("INFO: %d秒后自动重启 (第%d次重启)", restartDelay, status.Restarts))
			log.Printf("%d秒后自动重启进程 %s (第%d次重启)", restartDelay, name, status.Restarts)

			// 使用 goroutine 避免阻塞
			go func() {
				time.Sleep(time.Duration(restartDelay) * time.Second)
				err := pm.StartProcess(name)
				if err != nil {
					log.Printf("自动重启进程 %s 失败: %v", name, err)
				}
			}()
		}
	}
}

// addLog 添加日志
func (pm *ProcessManager) addLog(name, message string) {
	if status, exists := pm.processes[name]; exists {
		logLine := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), message)
		status.Output = append(status.Output, logLine)
		if len(status.Output) > 50 {
			status.Output = status.Output[1:]
		}
	}
}

// logWriter 用于捕获进程输出
type logWriter struct {
	name     string
	pm       *ProcessManager
	isStdout bool
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	line := strings.TrimSpace(string(p))
	if line == "" {
		return len(p), nil
	}

	lw.pm.mutex.Lock()
	defer lw.pm.mutex.Unlock()

	if status, exists := lw.pm.processes[lw.name]; exists {
		// 添加时间戳和类型标识
		prefix := "STDOUT"
		if !lw.isStdout {
			prefix = "STDERR"
		}
		logLine := fmt.Sprintf("[%s] %s: %s", time.Now().Format("15:04:05"), prefix, line)

		// 保留最近 50 行输出
		status.Output = append(status.Output, logLine)
		if len(status.Output) > 50 {
			status.Output = status.Output[1:]
		}

		// 也记录到主日志
		log.Printf("进程 %s %s: %s", lw.name, prefix, line)
	}

	return len(p), nil
}

// needsSudo 检查是否需要 sudo 权限
func needsSudo(command, user string) bool {
	// 如果指定了用户，需要 sudo
	if user != "" {
		return true
	}

	// 检查文件权限或者根据路径判断
	if strings.HasPrefix(command, "/opt/") || strings.HasPrefix(command, "/usr/") {
		return true
	}

	// 检查文件所有者
	if info, err := os.Stat(command); err == nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			// 如果文件属于 root 用户
			return stat.Uid == 0
		}
	}

	return false
}

// GetProcesses 获取所有进程状态
func (pm *ProcessManager) GetProcesses() map[string]*ProcessStatus {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	result := make(map[string]*ProcessStatus)
	for k, v := range pm.processes {
		// 创建副本避免并发问题
		statusCopy := *v
		result[k] = &statusCopy
	}
	return result
}

// ReloadConfig 重新加载配置
func (pm *ProcessManager) ReloadConfig() error {
	log.Println("重新加载配置文件...")
	return pm.LoadConfig()
}

// Web 处理器
func (pm *ProcessManager) handleIndex(w http.ResponseWriter, r *http.Request) {
	refreshTime := 10
	if pm.config != nil {
		refreshTime = pm.config.Server.RefreshTime
	}

	tmpl := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>LinkerBot Keeper</title>
    <meta charset="UTF-8">
    <meta http-equiv="refresh" content="%d">
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        table { width: 100%%; border-collapse: collapse; margin-top: 20px; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background-color: #f2f2f2; }
        .status-running { color: green; font-weight: bold; }
        .status-stopped { color: red; font-weight: bold; }
        .status-error { color: orange; font-weight: bold; }
        .status-disabled { color: gray; font-weight: bold; }
        button { padding: 8px 16px; margin: 2px; cursor: pointer; border: none; border-radius: 3px; }
        .btn-start { background-color: #4CAF50; color: white; }
        .btn-stop { background-color: #f44336; color: white; }
        .btn-restart { background-color: #2196F3; color: white; }
        .btn-enable { background-color: #FF9800; color: white; }
        .btn-logs { background-color: #9C27B0; color: white; }
        .btn-reload { background-color: #607D8B; color: white; }
        .refresh-btn { background-color: #FF9800; color: white; padding: 10px 20px; margin-bottom: 20px; }
        .info-box { background-color: #e7f3ff; border: 1px solid #b3d9ff; padding: 10px; margin-bottom: 20px; border-radius: 5px; }
        .config-info { background-color: #f0f8ff; border: 1px solid #b0d4f0; padding: 10px; margin-bottom: 20px; border-radius: 5px; }
        .loading { opacity: 0.6; pointer-events: none; }
        .description { font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <h1>进程管理器</h1>
    
    <div class="config-info">
        <strong>配置信息：</strong>
        <br>配置文件: %s
        <br>页面刷新间隔: %d秒
        <br><button class="btn-reload" onclick="reloadConfig()">重新加载配置</button>
    </div>
    
    <div class="info-box">
        <strong>说明：</strong>
        <ul>
            <li>页面每%d秒自动刷新</li>
            <li>进程重启超过配置的最大次数会自动禁用</li>
            <li>可以通过"启用重启"按钮重新启用并重置计数</li>
            <li>点击"日志"查看进程详细输出</li>
            <li>支持JSON和YAML配置文件格式</li>
        </ul>
    </div>
    
    <button class="refresh-btn" onclick="location.reload()">手动刷新</button>
    
    <table>
        <tr>
            <th>进程名称</th>
            <th>描述</th>
            <th>状态</th>
            <th>PID</th>
            <th>启动时间</th>
            <th>重启次数</th>
            <th>退出码</th>
            <th>最后错误</th>
            <th>操作</th>
        </tr>
        {{range $name, $status := .}}
        <tr>
            <td>
                <strong>{{$name}}</strong>
                <br><small>{{$status.Config.Command}}</small>
            </td>
            <td class="description">{{$status.Config.Description}}</td>
            <td class="status-{{$status.Status}}">{{$status.Status}}</td>
            <td>{{if ne $status.PID 0}}{{$status.PID}}{{else}}-{{end}}</td>
            <td>{{if not $status.StartTime.IsZero}}{{$status.StartTime.Format "2006-01-02 15:04:05"}}{{else}}-{{end}}</td>
            <td>{{$status.Restarts}}/{{$status.Config.MaxRestarts}}</td>
            <td>{{if ne $status.LastExitCode 0}}{{$status.LastExitCode}}{{else}}-{{end}}</td>
            <td title="{{$status.LastError}}">{{if $status.LastError}}{{printf "%%.30s" $status.LastError}}{{if gt (len $status.LastError) 30}}...{{end}}{{else}}-{{end}}</td>
            <td>
                {{if eq $status.Status "disabled"}}
                    <button class="btn-enable" onclick="controlProcess('{{$name}}', 'enable')">启用重启</button>
                {{else}}
                    <button class="btn-start" onclick="controlProcess('{{$name}}', 'start')" {{if eq $status.Status "running"}}disabled{{end}}>启动</button>
                    <button class="btn-stop" onclick="controlProcess('{{$name}}', 'stop')" {{if ne $status.Status "running"}}disabled{{end}}>停止</button>
                    <button class="btn-restart" onclick="controlProcess('{{$name}}', 'restart')">重启</button>
                {{end}}
                <button class="btn-logs" onclick="showLogs('{{$name}}')">日志</button>
            </td>
        </tr>
        {{end}}
    </table>

    <!-- 日志模态框 -->
    <div id="logModal" style="display:none; position:fixed; top:0; left:0; width:100%%; height:100%%; background-color:rgba(0,0,0,0.7); z-index:1000;">
        <div style="position:relative; margin:2%% auto; width:90%%; background-color:white; padding:20px; border-radius:5px; max-height:90%%; overflow-y:auto;">
            <h3 id="logTitle">进程日志</h3>
            <button onclick="closeLogModal()" style="float:right; margin-top:-40px; padding:5px 10px;">关闭</button>
            <pre id="logContent" style="background-color:#f5f5f5; padding:15px; border-radius:3px; max-height:500px; overflow-y:auto; font-size:12px; line-height:1.4;"></pre>
        </div>
    </div>

    <script>
        function controlProcess(name, action) {
            // 添加加载状态
            const buttons = document.querySelectorAll('button');
            buttons.forEach(btn => btn.classList.add('loading'));
            
            let url = '/api/process/' + name + '/' + action;
            if (action === 'enable') {
                url = '/api/enable/' + name;
            }
            
            fetch(url, {
                method: 'POST'
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    alert('操作成功: ' + data.message);
                    setTimeout(() => location.reload(), 1000);
                } else {
                    alert('操作失败: ' + data.error);
                    buttons.forEach(btn => btn.classList.remove('loading'));
                }
            })
            .catch(error => {
                alert('请求失败: ' + error);
                buttons.forEach(btn => btn.classList.remove('loading'));
            });
        }

        function reloadConfig() {
            const buttons = document.querySelectorAll('button');
            buttons.forEach(btn => btn.classList.add('loading'));
            
            fetch('/api/reload', {
                method: 'POST'
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    alert('配置重新加载成功: ' + data.message);
                    setTimeout(() => location.reload(), 1000);
                } else {
                    alert('配置重新加载失败: ' + data.error);
                    buttons.forEach(btn => btn.classList.remove('loading'));
                }
            })
            .catch(error => {
                alert('请求失败: ' + error);
                buttons.forEach(btn => btn.classList.remove('loading'));
            });
        }

        function showLogs(name) {
            fetch('/api/logs/' + name)
            .then(response => response.json())
            .then(data => {
                document.getElementById('logTitle').textContent = '进程 ' + name + ' 的日志';
                const logs = data.logs || [];
                if (logs.length === 0) {
                    document.getElementById('logContent').textContent = '暂无日志记录';
                } else {
                    document.getElementById('logContent').textContent = logs.join('\\n');
                }
                document.getElementById('logModal').style.display = 'block';
            })
            .catch(error => {
                alert('获取日志失败: ' + error);
            });
        }

        function closeLogModal() {
            document.getElementById('logModal').style.display = 'none';
        }

        // 点击模态框外部关闭
        window.onclick = function(event) {
            const modal = document.getElementById('logModal');
            if (event.target === modal) {
                modal.style.display = 'none';
            }
        }
    </script>
</body>
</html>`, refreshTime, pm.configPath, refreshTime, refreshTime)

	t := template.Must(template.New("index").Parse(tmpl))
	processes := pm.GetProcesses()
	t.Execute(w, processes)
}

// API 处理器
func (pm *ProcessManager) handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 解析路径：/api/process/name/action
	path := r.URL.Path[len("/api/process/"):]
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "无效的 API 路径",
		})
		return
	}

	name := parts[0]
	action := parts[1]

	var err error
	var message string

	switch action {
	case "start":
		err = pm.StartProcess(name)
		message = fmt.Sprintf("进程 %s 启动成功", name)
	case "stop":
		err = pm.StopProcess(name)
		message = fmt.Sprintf("进程 %s 停止成功", name)
	case "restart":
		err = pm.RestartProcess(name)
		message = fmt.Sprintf("进程 %s 重启成功", name)
	default:
		err = fmt.Errorf("未知操作: %s", action)
	}

	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": message,
		})
	}
}

// 启用自动重启 API
func (pm *ProcessManager) handleEnable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := r.URL.Path[len("/api/enable/"):]

	err := pm.EnableAutoRestart(name)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("进程 %s 已启用自动重启", name),
		})
	}
}

// 重新加载配置 API
func (pm *ProcessManager) handleReload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := pm.ReloadConfig()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "配置重新加载成功",
		})
	}
}

// 日志 API
func (pm *ProcessManager) handleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := r.URL.Path[len("/api/logs/"):]

	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	if status, exists := pm.processes[name]; exists {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"logs":    status.Output,
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "进程不存在",
		})
	}
}

// 状态 API
func (pm *ProcessManager) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	processes := pm.GetProcesses()
	json.NewEncoder(w).Encode(processes)
}

// 配置 API
func (pm *ProcessManager) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	pm.mutex.RLock()
	config := pm.config
	pm.mutex.RUnlock()

	if config == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "配置未加载",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"config":  config,
	})
}

func main() {
	// 解析命令行参数
	configPath := "keeper.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	pm := NewProcessManager(configPath)

	// 加载配置
	err := pm.LoadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 检查可执行文件是否存在
	log.Println("检查可执行文件...")
	for name, status := range pm.GetProcesses() {
		execPath := status.Config.Command
		if filepath.IsAbs(execPath) {
			if _, err := os.Stat(execPath); os.IsNotExist(err) {
				log.Printf("警告: 可执行文件 %s 不存在，进程 %s 将无法启动", execPath, name)
			} else {
				log.Printf("发现可执行文件: %s", execPath)
			}
		} else {
			if _, err := exec.LookPath(execPath); err != nil {
				log.Printf("警告: 命令 %s 在PATH中不存在，进程 %s 将无法启动", execPath, name)
			} else {
				log.Printf("发现命令: %s", execPath)
			}
		}
	}

	// 启动所有启用的进程
	for name, status := range pm.GetProcesses() {
		if status.Config.Enabled {
			go func(processName string) {
				time.Sleep(2 * time.Second) // 延迟启动
				err := pm.StartProcess(processName)
				if err != nil {
					log.Printf("启动进程 %s 失败: %v", processName, err)
				}
			}(name)
		}
	}

	// 定期检查配置文件变化
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := pm.LoadConfig()
				if err != nil {
					log.Printf("定期加载配置失败: %v", err)
				}
			}
		}
	}()

	// 设置 Web 路由
	http.HandleFunc("/", pm.handleIndex)
	http.HandleFunc("/api/process/", pm.handleAPI)
	http.HandleFunc("/api/enable/", pm.handleEnable)
	http.HandleFunc("/api/reload", pm.handleReload)
	http.HandleFunc("/api/logs/", pm.handleLogs)
	http.HandleFunc("/api/status", pm.handleStatus)
	http.HandleFunc("/api/config", pm.handleConfig)

	// 启动 Web 服务器
	address := "0.0.0.0:8080"
	if pm.config != nil {
		address = fmt.Sprintf("%s:%s", pm.config.Server.Host, pm.config.Server.Port)
	}

	log.Printf("进程管理器启动")
	log.Printf("配置文件: %s", configPath)
	log.Printf("Web界面: http://%s", address)
	log.Fatal(http.ListenAndServe(address, nil))
}
