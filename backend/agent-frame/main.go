package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/jettjia/xiaoqinglong/agent-frame/api/event"
	"github.com/jettjia/xiaoqinglong/agent-frame/api/grpc"
	"github.com/jettjia/xiaoqinglong/agent-frame/api/http"
	"github.com/jettjia/xiaoqinglong/agent-frame/api/job"
	"github.com/jettjia/xiaoqinglong/agent-frame/application/service/agent"
	"github.com/jettjia/xiaoqinglong/agent-frame/boot"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po"
)

func main() {
	// create shutdown channel
	shutdown := make(chan struct{})

	// init config
	env := flag.String("env", "debug", "configure environment reading")
	flag.Parse()

	err := os.Setenv("env", *env)
	if err != nil {
		panic(err)
	}

	// auto create table
	if err = po.AutoTable(); err != nil {
		panic(err)
	}

	// init data
	if err := boot.InitData(); err != nil {
		panic(err)
	}

	// start http
	http.InitHttp()

	// start grpc
	grpc.InitGrpc()

	// start mcp
	// mcp.InitMCP()

	// start event mq
	event.InitEvent()

	// start InitJob
	go func() {
		job.InitJob(shutdown)
	}()

	// 恢复 JobManager 中的周期任务（从数据库同步）
	go func() {
		agentSvc := agent.NewSysAgentService()
		agents, err := agentSvc.FindPeriodicAgents(context.Background())
		if err != nil {
			// log error but don't panic - the service can still run
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// close
	close(shutdown)
}
