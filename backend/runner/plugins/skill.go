package plugins

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/jettjia/XiaoQinglong/runner/backend"
)

// NewSkillMiddleware creates a skill middleware using eino official implementation
// Skills are loaded from SKILL.md files in the skillsDir directory
func NewSkillMiddleware(ctx context.Context, skillsDir string) (adk.ChatModelAgentMiddleware, error) {
	fsBackend := backend.NewLocalFSBackend()

	skillBackend, err := skill.NewBackendFromFilesystem(ctx, &skill.BackendFromFilesystemConfig{
		Backend: fsBackend,
		BaseDir: skillsDir,
	})
	if err != nil {
		return nil, err
	}

	return skill.NewMiddleware(ctx, &skill.Config{
		Backend: skillBackend,
	})
}
