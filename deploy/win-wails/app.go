package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jettjia/xiaoqinglong/agent-frame/api/event"
	"github.com/jettjia/xiaoqinglong/agent-frame/api/http"
	"github.com/jettjia/xiaoqinglong/agent-frame/api/job"
	"github.com/jettjia/xiaoqinglong/agent-frame/application/service/agent"
	"github.com/jettjia/xiaoqinglong/agent-frame/boot"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po"
)

var runnerProcess *exec.Cmd

func init() {
	// 初始化日志到文件
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	logDir := filepath.Join(home, ".xiaoqinglong", "logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "wails.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(f)
	}
}

func startup() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Wails] PANIC recovered in startup: %v", r)
		}
	}()

	log.Println("[Wails] startup() called")
	log.Printf("[Wails] Executable path: %s", getExecutablePath())

	// 设置环境变量
	os.Setenv("env", "debug")

	// 资源已在 init() 中释放到 ~/.xiaoqinglong/
	// 设置 skills 目录
	baseDir := getBaseDir()
	skillsPath := filepath.Join(baseDir, "skills")
	os.Setenv("XQL_SOURCE_SKILLS_DIR", skillsPath)
	log.Printf("[Wails] XQL_SOURCE_SKILLS_DIR: %s", skillsPath)

	// 设置配置文件路径
	configPath := filepath.Join(baseDir, "config", "config.yaml")
	os.Setenv("XQL_CONFIG_PATH", configPath)
	log.Printf("[Wails] XQL_CONFIG_PATH: %s", configPath)

	// 查找 runner.exe（从 ~/.xiaoqinglong/runner.exe）
	runnerPath := filepath.Join(baseDir, "runner.exe")
	log.Printf("[Wails] Runner path: %s", runnerPath)

	// 检查 runner 是否存在
	if _, err := os.Stat(runnerPath); os.IsNotExist(err) {
		log.Printf("[Wails] runner.exe not found at %s", runnerPath)
	} else {
		// 启动 runner 作为子进程
		log.Println("[Wails] Starting runner subprocess...")
		runnerProcess = exec.Command(runnerPath)
		if err := runnerProcess.Start(); err != nil {
			log.Printf("[Wails] Failed to start runner: %v", err)
		} else {
			log.Println("[Wails] Runner subprocess started")
		}
	}

	// 等待 runner 启动
	time.Sleep(2 * time.Second)

	log.Println("[Wails] Initializing database...")

	// 初始化 agent-frame 数据库
	log.Println("[Wails] Calling po.AutoTable()...")
	if err := po.AutoTable(); err != nil {
		log.Printf("[Wails] AutoTable error: %v", err)
	} else {
		log.Println("[Wails] AutoTable done")
	}

	// 初始化目录
	log.Println("[Wails] Calling boot.InitDirs()...")
	if err := boot.InitDirs(); err != nil {
		log.Printf("[Wails] InitDirs error: %v", err)
	} else {
		log.Println("[Wails] InitDirs done")
	}

	// 初始化数据
	log.Println("[Wails] Calling boot.InitData()...")
	if err := boot.InitData(); err != nil {
		log.Printf("[Wails] InitData error: %v", err)
	} else {
		log.Println("[Wails] InitData done")
	}

	// 启动 HTTP 服务
	log.Println("[Wails] Starting agent-frame HTTP on :9292...")
	http.InitHttp()
	log.Println("[Wails] HTTP server started")

	// 启动 channel websocket connections
	if err := boot.StartChannelWsConnections(); err != nil {
		log.Printf("[Wails] Failed to start channel WS connections: %v", err)
	}

	// 启动事件队列
	event.InitEvent()

	// 启动定时任务
	shutdownChan := make(chan struct{})
	go func() {
		job.InitJob(shutdownChan)
	}()

	// 恢复定时任务
	go func() {
		agentSvc := agent.NewSysAgentService()
		agents, err := agentSvc.FindPeriodicAgents(context.Background())
		if err != nil {
			log.Printf("[Wails] FindPeriodicAgents error: %v", err)
		} else {
			periodicAgents := make([]job.PeriodicAgent, 0, len(agents))
			for _, ag := range agents {
				periodicAgents = append(periodicAgents, job.PeriodicAgent{
					Ulid:       ag.Ulid,
					Name:       ag.Name,
					CronRule:   ag.CronRule,
					ConfigJson: ag.ConfigJson,
					Enabled:    ag.Enabled,
				})
			}
			job.SyncCronJobsFromDB(periodicAgents)
		}
	}()

	log.Println("[Wails] XiaoQingLong started successfully!")
	log.Println("[Wails] Waiting for WebView2 to load frontend...")
}

func shutdown() {
	log.Println("[Wails] XiaoQingLong shutting down...")

	// 停止 runner 子进程
	if runnerProcess != nil && runnerProcess.Process != nil {
		log.Println("[Wails] Stopping runner subprocess...")
		runnerProcess.Process.Kill()
		runnerProcess.Wait()
		log.Println("[Wails] Runner subprocess stopped")
	}

	log.Println("[Wails] XiaoQingLong stopped")
}

func getExecutablePath() string {
	execPath, _ := os.Executable()
	return execPath
}

func getBaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".xiaoqinglong")
}
