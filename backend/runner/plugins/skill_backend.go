package plugins

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/adk/middlewares/skill"
	"gopkg.in/yaml.v3"
)

// SkillBackend 实现 eino skill.Backend 接口
// 从本地文件系统读取 SKILL.md 文件
type SkillBackend struct {
	baseDir string
}

// NewSkillBackend 创建 SkillBackend
func NewSkillBackend(baseDir string) *SkillBackend {
	return &SkillBackend{
		baseDir: baseDir,
	}
}

// List 返回所有 skill 的 FrontMatter
func (b *SkillBackend) List(ctx context.Context) ([]skill.FrontMatter, error) {
	skills, err := b.loadAll()
	if err != nil {
		return nil, err
	}

	matters := make([]skill.FrontMatter, 0, len(skills))
	for _, s := range skills {
		matters = append(matters, s.FrontMatter)
	}
	return matters, nil
}

// Get 返回单个 Skill
func (b *SkillBackend) Get(ctx context.Context, name string) (skill.Skill, error) {
	skills, err := b.loadAll()
	if err != nil {
		return skill.Skill{}, err
	}

	s, ok := skills[name]
	if !ok {
		return skill.Skill{}, nil
	}
	return *s, nil
}

// loadAll 加载所有 skills
func (b *SkillBackend) loadAll() (map[string]*skill.Skill, error) {
	skills := make(map[string]*skill.Skill)

	entries, err := os.ReadDir(b.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return skills, nil // 目录不存在，返回空
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(b.baseDir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue // 没有 SKILL.md 或读取失败，跳过
		}

		s, err := b.parseSkill(string(data), filepath.Dir(skillPath))
		if err != nil || s == nil {
			continue // 解析失败，跳过
		}

		skills[s.Name] = s
	}

	return skills, nil
}

// parseSkill 解析 SKILL.md 内容
func (b *SkillBackend) parseSkill(data string, dir string) (*skill.Skill, error) {
	frontmatter, content, err := parseSkillFrontmatter(data)
	if err != nil {
		return nil, err
	}

	var fm skill.FrontMatter
	if err = yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return nil, err
	}

	if fm.Name == "" {
		return nil, nil
	}

	return &skill.Skill{
		FrontMatter:   fm,
		Content:       strings.TrimSpace(content),
		BaseDirectory: dir,
	}, nil
}

// parseSkillFrontmatter 解析 YAML frontmatter 和 markdown content
func parseSkillFrontmatter(data string) (frontmatter string, content string, err error) {
	const delimiter = "---"

	data = strings.TrimSpace(data)

	if !strings.HasPrefix(data, delimiter) {
		return "", "", nil
	}

	rest := data[len(delimiter):]
	endIdx := strings.Index(rest, "\n"+delimiter)
	if endIdx == -1 {
		return "", "", nil
	}

	frontmatter = strings.TrimSpace(rest[:endIdx])
	content = rest[endIdx+len("\n"+delimiter):]

	if strings.HasPrefix(content, "\n") {
		content = content[1:]
	}

	return frontmatter, content, nil
}
