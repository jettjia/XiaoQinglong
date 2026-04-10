package router

import (
	"github.com/gin-gonic/gin"

	publicRouter "github.com/jettjia/xiaoqinglong/agent-frame/api/http/router/public"
)

func Routers(engine *gin.Engine) *gin.Engine {
	// 注册路由
	ApiGroup := engine.Group("/api/xiaoqinglong/agent-frame/v1")
	publicRouter.SetPublicRouter(ApiGroup) // router
	return engine
}
