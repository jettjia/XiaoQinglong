package boot

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
	dtoSkill "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/skill"
	"github.com/jettjia/xiaoqinglong/agent-frame/application/service/skill"
	srvSkill "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/skill"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/xqldir"
)

// getSourceSkillsPath 获取源码仓库中的 skills 目录路径
// 这是初始skill包的来源
func getSourceSkillsPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "../../skills"
	}
	// 当前工作目录是 backend/agent-frame，skills 在项目根目录
	return filepath.Join(cwd, "..", "..", "skills")
}

// getTargetSkillsPath 获取统一目录下的 skills 路径
// 这是运行时实际使用的 skills 目录
func getTargetSkillsPath() string {
	return xqldir.GetSkillsDir()
}

// syncSkillsFromDisk 扫描源码 skills 目录，同步到统一目录并入库
func syncSkillsFromDisk() error {
	sourceRoot := getSourceSkillsPath()
	targetRoot := getTargetSkillsPath()

	log.Println("[Init] Initializing system skills")
	log.Println("[Init] Source skills directory:", sourceRoot)
	log.Println("[Init] Target skills directory:", targetRoot)

	// 确保目标目录存在
	if err := os.MkdirAll(targetRoot, 0755); err != nil {
		return err
	}

	// 检查源码 skills 目录是否存在
	info, err := os.Stat(sourceRoot)
	if err != nil {
		if os.IsNotExist(err) {
			// 源码 skills 目录不存在，跳过（统一目录中可能已有）
			log.Println("[Init] Source skills directory not found, skipping copy")
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	// 读取源码 skills 目录下的所有子目录
	entries, err := os.ReadDir(sourceRoot)
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
		sourcePath := filepath.Join(sourceRoot, skillName)
		targetPath := filepath.Join(targetRoot, skillName)
		skillMdPath := filepath.Join(sourcePath, "SKILL.md")

		// 解析 SKILL.md 获取元信息
		skillType, description, version := parseSkillMd(skillMdPath)
		if skillType == "" {
			// 没有 SKILL.md 或解析失败，默认设为 skill 类型
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

		// 如果数据库已存在，跳过拷贝（但仍更新一下 Path 以防万一）
		if len(existing) > 0 {
			// 确保统一目录中有这个 skill
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				log.Printf("[Init] Copying skill '%s' to unified directory", skillName)
				if err := copyDir(sourcePath, targetPath); err != nil {
					log.Printf("[Init] Failed to copy skill '%s': %v", skillName, err)
					continue
				}
			}
			continue
		}

		// 拷贝到统一目录
		log.Printf("[Init] Copying skill '%s' to unified directory", skillName)
		if err := copyDir(sourcePath, targetPath); err != nil {
			log.Printf("[Init] Failed to copy skill '%s': %v", skillName, err)
			continue
		}

		// 创建为系统内置 skill，Path 指向统一目录
		createReq := &dtoSkill.CreateSysSkillReq{
			Name:        skillName,
			Description: description,
			SkillType:   skillType,
			Version:     version,
			Path:        targetPath, // 使用统一目录路径
			Enabled:     true,
			Config:      "{}",
			IsSystem:    true, // 系统内置
		}

		_, err = skillSvc.CreateSysSkill(ctx, createReq)
		if err != nil {
			// 忽略创建错误，继续处理其他 skill
			log.Printf("[Init] Failed to create skill '%s' in DB: %v", skillName, err)
			continue
		}
	}

	return nil
}

// copyDir 拷贝目录到目标位置
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// 创建目标目录
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile 拷贝单个文件
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
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
