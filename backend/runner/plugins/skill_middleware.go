package plugins

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
)

// NewSkillMiddleware 创建 skill middleware
// 使用 eino 官方的 skill middleware 从本地文件系统加载 SKILL.md
func NewSkillMiddleware(ctx context.Context, skillsDir string) (adk.ChatModelAgentMiddleware, error) {
	backend := NewSkillBackend(skillsDir)

	mw, err := skill.NewMiddleware(ctx, &skill.Config{
		Backend: backend,
	})
	if err != nil {
		return nil, err
	}

	return mw, nil
}
