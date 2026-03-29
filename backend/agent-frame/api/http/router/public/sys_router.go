package public

import (
	"github.com/gin-gonic/gin"

	handUser "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/user"
	handConfig "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/config"
	handModel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/model"
	handKnowledgeBase "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/knowledge_base"
	handSkill "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/skill"
	handAgent "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/agent"
)

func SetPublicRouter(Router *gin.RouterGroup) {
	handUser := handUser.NewHandler()
	handConfig := handConfig.NewHandler()
	handModel := handModel.NewHandler()
	handKnowledgeBase := handKnowledgeBase.NewHandler()
	handSkill := handSkill.NewHandler()
	handAgent := handAgent.NewHandler()

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

	// knowledge_base
	KBRouter := Router.Group("/knowledge_base")
	{
		KBRouter.POST("", handKnowledgeBase.CreateSysKnowledgeBase)              // 创建
		KBRouter.DELETE("/:ulid", handKnowledgeBase.DeleteSysKnowledgeBase)     // 删除
		KBRouter.PUT("/:ulid", handKnowledgeBase.UpdateSysKnowledgeBase)        // 修改
		KBRouter.GET("/:ulid", handKnowledgeBase.FindSysKnowledgeBaseById)      // 查询ByID
		KBRouter.POST("/all", handKnowledgeBase.FindSysKnowledgeBaseAll)         // 查询所有
		KBRouter.POST("/page", handKnowledgeBase.FindSysKnowledgeBasePage)      // 分页查询
		KBRouter.POST("/:ulid/recall", handKnowledgeBase.RecallTest)            // 召回测试
	}

	// skill
	SkillRouter := Router.Group("/skill")
	{
		SkillRouter.POST("", handSkill.CreateSysSkill)              // 创建
		SkillRouter.DELETE("/:ulid", handSkill.DeleteSysSkill)     // 删除
		SkillRouter.PUT("/:ulid", handSkill.UpdateSysSkill)        // 修改
		SkillRouter.GET("/:ulid", handSkill.FindSysSkillById)      // 查询ByID
		SkillRouter.POST("/all", handSkill.FindSysSkillAll)         // 查询所有
		SkillRouter.POST("/page", handSkill.FindSysSkillPage)      // 分页查询
		SkillRouter.POST("/upload", handSkill.UploadSysSkill)      // 上传安装
		SkillRouter.POST("/check-name", handSkill.CheckSkillName)  // 检查同名
	}

	// agent
	AgentRouter := Router.Group("/agent")
	{
		AgentRouter.POST("", handAgent.CreateSysAgent)              // 创建
		AgentRouter.DELETE("/:ulid", handAgent.DeleteSysAgent)     // 删除
		AgentRouter.PUT("/:ulid", handAgent.UpdateSysAgent)        // 修改
		AgentRouter.GET("/:ulid", handAgent.FindSysAgentById)      // 查询ByID
		AgentRouter.POST("/all", handAgent.FindSysAgentAll)         // 查询所有
		AgentRouter.POST("/page", handAgent.FindSysAgentPage)      // 分页查询
		AgentRouter.POST("/upload", handAgent.UploadSysAgent)      // 上传导入
	}
}
