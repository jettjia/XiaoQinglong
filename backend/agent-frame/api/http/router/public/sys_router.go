package public

import (
	"github.com/gin-gonic/gin"

	handUser "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/user"
	handConfig "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/config"
	handModel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/model"
)

func SetPublicRouter(Router *gin.RouterGroup) {
	handUser := handUser.NewHandler()
	handConfig := handConfig.NewHandler()
	handModel := handModel.NewHandler()

	GRouter := Router.Group("/user")
	{

		// user
		GRouter.POST("/user", handUser.CreateSysUser)              // 创建
		GRouter.DELETE("/user/:ulid", handUser.DeleteSysUser)      // 删除
		GRouter.PUT("/user/:ulid", handUser.UpdateSysUser)         // 修改
		GRouter.GET("/user/:ulid", handUser.FindSysUserById)       // 查询ByID
		GRouter.POST("/user/byQuery", handUser.FindSysUserByQuery) // 查询ByQuery
		GRouter.POST("/user/byAll", handUser.FindSysUserAll)       // 查询ByAll
		GRouter.POST("/userPage", handUser.FindSysUserPage)        // 查询分页
	}

	// config
	CRouter := Router.Group("/config")
	{
		CRouter.GET("/app", handConfig.GetAppConfig)               // 获取应用配置
		CRouter.PUT("/app", handConfig.SaveAppConfig)             // 保存应用配置
		CRouter.GET("/skills", handConfig.GetSkillsConfig)        // 获取技能配置
		CRouter.PUT("/skills", handConfig.SaveSkillsConfig)       // 保存技能配置
	}

	// model
	MRouter := Router.Group("/model")
	{
		MRouter.POST("", handModel.CreateSysModel)              // 创建
		MRouter.DELETE("/:ulid", handModel.DeleteSysModel)     // 删除
		MRouter.PUT("/:ulid", handModel.UpdateSysModel)        // 修改
		MRouter.GET("/:ulid", handModel.FindSysModelById)      // 查询ByID
		MRouter.POST("/all", handModel.FindSysModelAll)         // 查询所有
		MRouter.POST("/page", handModel.FindSysModelPage)       // 分页查询
	}
}
