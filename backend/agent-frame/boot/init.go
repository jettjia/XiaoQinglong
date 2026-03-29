package boot

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	dtoSkill "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/skill"
	"github.com/jettjia/xiaoqinglong/agent-frame/application/service/skill"
	srvSkill "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/skill"
)

func InitData() (err error) {
	// 初始化扫描 skills 目录并同步到数据库
	if err = syncSkillsFromDisk(); err != nil {
		return
	}

	return
}

// getSkillsPath 获取 skills 目录路径
// 使用相对于工作目录的路径
func getSkillsPath() string {
	// 使用当前工作目录
	cwd, err := os.Getwd()
	if err != nil {
		return "../../skills"
	}
	// 当前工作目录是 backend/agent-frame，skills 在项目根目录
	return filepath.Join(cwd, "..", "..", "skills")
}

// syncSkillsFromDisk 扫描 skills 目录，同步到数据库
func syncSkillsFromDisk() error {
	// 从配置获取 skills 目录路径，默认为 ../../skills

	log.Println("[Init] Initializing system skills, scanning directory:", getSkillsPath())

	skillsRoot := getSkillsPath()
	if skillsRoot == "" {
		return nil
	}

	// 检查 skills 目录是否存在
	info, err := os.Stat(skillsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			// skills 目录不存在，跳过
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	// 读取 skills 目录下的所有子目录
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return err
	}

	skillSvc := skill.NewSysSkillService()
	domainSkillSvc := srvSkill.NewSysSkillSvc()
	ctx := context.Background()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillPath := filepath.Join(skillsRoot, skillName)
		skillMdPath := filepath.Join(skillPath, "SKILL.md")

		// 解析 SKILL.md 获取元信息
		skillType, description, version := parseSkillMd(skillMdPath)
		if skillType == "" {
			// 没有 SKILL.md 或解析失败，默认设为 tool 类型
			skillType = "skill"
		}

		// 检查数据库中是否已存在同名且同类型的 skill
		queries := []*builder.Query{
			{Key: "name", Operator: builder.Operator_opEq, Value: skillName},
			{Key: "skill_type", Operator: builder.Operator_opEq, Value: skillType},
			{Key: "deleted_at", Operator: builder.Operator_opEq, Value: 0},
		}

		existing, err := domainSkillSvc.FindAll(ctx, queries)
		if err != nil {
			return err
		}

		if len(existing) > 0 {
			// 已存在，跳过
			continue
		}

		// 不存在，创建为系统内置 skill
		createReq := &dtoSkill.CreateSysSkillReq{
			Name:        skillName,
			Description: description,
			SkillType:   skillType,
			Version:     version,
			Path:        skillPath,
			Enabled:     true,
			Config:      "{}",
			IsSystem:    true, // 系统内置
		}

		_, err = skillSvc.CreateSysSkill(ctx, createReq)
		if err != nil {
			// 忽略创建错误，继续处理其他 skill
			continue
		}
	}

	return nil
}

// parseSkillMd 解析 SKILL.md 提取元信息
func parseSkillMd(skillMdPath string) (skillType, description, version string) {
	data, err := os.ReadFile(skillMdPath)
	if err != nil {
		return "", "", ""
	}

	content := string(data)

	// 简单解析 YAML front matter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			yamlContent := parts[1]
			lines := strings.Split(yamlContent, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "name:") {
					// name 字段忽略，使用目录名
				} else if strings.HasPrefix(line, "description:") {
					description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				} else if strings.HasPrefix(line, "type:") {
					skillType = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
				} else if strings.HasPrefix(line, "version:") {
					version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
				}
			}
		}
	}

	return
}

type InitHandler struct {
}

func NewInitHandler() *InitHandler {
	return &InitHandler{}
}
