package skill

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	ass "github.com/jettjia/xiaoqinglong/agent-frame/application/assembler/skill"
	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/skill"
	srv "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/skill"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/xqldir"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

type SysSkillService struct {
	sysSkillDto *ass.SysSkillDto
	sysSkillSrv *srv.SysSkillSvc
	skillsRoot  string // skills 根目录
}

func NewSysSkillService() *SysSkillService {
	return &SysSkillService{
		sysSkillDto: ass.NewSysSkillDto(),
		sysSkillSrv: srv.NewSysSkillSvc(),
		skillsRoot:  xqldir.GetSkillsDir(), // 使用统一目录
	}
}

func (s *SysSkillService) CreateSysSkill(ctx context.Context, req *dto.CreateSysSkillReq) (*dto.CreateSysSkillRsp, error) {
	var rsp dto.CreateSysSkillRsp
	en := s.sysSkillDto.D2ECreateSysSkill(req)

	ulid, err := s.sysSkillSrv.Create(ctx, en)
	if err != nil {
		return nil, err
	}
	rsp.Ulid = ulid

	return &rsp, nil
}

func (s *SysSkillService) DeleteSysSkill(ctx context.Context, req *dto.DelSysSkillReq) error {
	// 检查是否为系统内置技能
	existing, err := s.sysSkillSrv.FindById(ctx, req.Ulid)
	if err != nil {
		return err
	}
	if existing == nil || existing.DeletedAt != 0 {
		return xerror.NewErrorOpt(apierror.NotFoundErr, xerror.WithCause("skill not found"))
	}
	if existing.IsSystem {
		return xerror.NewErrorOpt(apierror.ForbiddenErr, xerror.WithCause("系统内置技能不能删除"))
	}

	en := s.sysSkillDto.D2EDeleteSysSkill(req)

	return s.sysSkillSrv.Delete(ctx, en)
}

func (s *SysSkillService) UpdateSysSkill(ctx context.Context, req *dto.UpdateSysSkillReq) error {
	en := s.sysSkillDto.D2EUpdateSysSkill(req)

	return s.sysSkillSrv.Update(ctx, en)
}

func (s *SysSkillService) FindSysSkillById(ctx context.Context, req *dto.FindSysSkillByIdReq) (*dto.FindSysSkillRsp, error) {
	en, err := s.sysSkillSrv.FindById(ctx, req.Ulid)
	if err != nil {
		return nil, err
	}

	// 过滤已删除的记录
	if en == nil || en.DeletedAt != 0 {
		return nil, xerror.NewErrorOpt(apierror.NotFoundErr, xerror.WithCause("skill not found or deleted"))
	}

	dtoRsp := s.sysSkillDto.E2DFindSysSkillRsp(en)

	return dtoRsp, nil
}

func (s *SysSkillService) FindSysSkillAll(ctx context.Context, req *dto.FindSysSkillAllReq) ([]*dto.FindSysSkillRsp, error) {
	queries := []*builder.Query{
		{Key: "deleted_at", Operator: builder.Operator_opEq, Value: 0},
	}

	if req.SkillType != "" {
		queries = append(queries, &builder.Query{Key: "skill_type", Operator: builder.Operator_opEq, Value: req.SkillType})
	}
	if req.Name != "" {
		queries = append(queries, &builder.Query{Key: "name", Operator: builder.Operator_opLike, Value: req.Name})
	}

	ens, err := s.sysSkillSrv.FindAll(ctx, queries)
	if err != nil {
		return nil, err
	}

	dtos := s.sysSkillDto.E2DGetSysSkills(ens)

	return dtos, nil
}

func (s *SysSkillService) FindSysSkillPage(ctx context.Context, req *dto.FindSysSkillPageReq) (*dto.FindSysSkillPageRsp, error) {
	var rsp dto.FindSysSkillPageRsp
	ens, pageData, err := s.sysSkillSrv.FindPage(ctx, req.Query, req.PageData, req.SortData)
	if err != nil {
		return nil, err
	}

	entries := s.sysSkillDto.E2DGetSysSkills(ens)
	rsp.Entries = entries
	rsp.PageData = pageData

	return &rsp, nil
}

// CheckSkillName 检查同名 Skill 是否存在
func (s *SysSkillService) CheckSkillName(ctx context.Context, name string) (*dto.CheckSkillNameRsp, error) {
	skill, err := s.sysSkillSrv.FindByName(ctx, name)
	if err != nil {
		return nil, err
	}

	rsp := &dto.CheckSkillNameRsp{
		Exists:  false,
		Message: "",
	}

	// 软删除的记录不视为冲突
	if skill != nil && skill.DeletedAt == 0 {
		rsp.Exists = true
		rsp.Message = fmt.Sprintf("Skill '%s' 已存在，是否覆盖？", name)
	}

	return rsp, nil
}

// UploadSysSkill 上传并安装 Skill
func (s *SysSkillService) UploadSysSkill(ctx context.Context, fileData []byte, fileName string, createdBy string) (*dto.FindSysSkillRsp, error) {
	// 解压 ZIP
	skillsDir, skillInfo, err := s.extractSkillZip(fileData, fileName)
	if err != nil {
		return nil, err
	}

	// 检查同名且同类型
	existing, err := s.sysSkillSrv.FindByNameAndType(ctx, skillInfo.Name, skillInfo.SkillType)
	if err != nil {
		return nil, err
	}

	if existing != nil && existing.DeletedAt == 0 {
		// 已存在同名且同类型的 skill
		return nil, xerror.NewErrorOpt(apierror.ConflictErr, xerror.WithCause(fmt.Sprintf("Skill '%s' (类型: %s) 已存在", skillInfo.Name, skillInfo.SkillType)))
	}

	// 创建数据库记录
	createReq := &dto.CreateSysSkillReq{
		CreatedBy:   createdBy,
		Name:        skillInfo.Name,
		Description: skillInfo.Description,
		SkillType:   skillInfo.SkillType,
		Version:     skillInfo.Version,
		Path:        skillsDir,
		Enabled:     true,
		Config:      "{}",
		IsSystem:    false, // 用户上传的不是系统内置
	}

	rsp, err := s.CreateSysSkill(ctx, createReq)
	if err != nil {
		return nil, err
	}

	// 返回完整记录
	return s.FindSysSkillById(ctx, &dto.FindSysSkillByIdReq{Ulid: rsp.Ulid})
}

// SkillMeta 解析 SKILL.md 提取的元信息
type SkillMeta struct {
	Name        string
	Description string
	SkillType   string // tool/mcp/a2a
	Version     string
}

// extractSkillZip 解压 Skill ZIP 包
func (s *SysSkillService) extractSkillZip(fileData []byte, fileName string) (string, *SkillMeta, error) {
	reader, err := zip.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return "", nil, fmt.Errorf("invalid zip file: %w", err)
	}

	var skillMeta *SkillMeta
	var skillRootDir string

	// 第一遍：找到顶层目录名和 SKILL.md
	for _, f := range reader.File {
		// 获取顶层目录
		parts := strings.Split(f.Name, "/")
		if len(parts) > 1 && skillRootDir == "" {
			skillRootDir = parts[0]
		}
		if skillRootDir != "" && len(parts) > 1 && parts[1] == "SKILL.md" {
			break
		}
	}

	if skillRootDir == "" {
		return "", nil, fmt.Errorf("invalid skill package: no root directory found")
	}

	// 解压到 skills/{skillRootDir}
	targetDir := filepath.Join(s.skillsRoot, skillRootDir)

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create skill directory: %w", err)
	}

	// 解压所有文件
	for _, f := range reader.File {
		if err := s.extractFile(f, targetDir); err != nil {
			return "", nil, err
		}
	}

	// 解析 SKILL.md
	skillMdPath := filepath.Join(targetDir, "SKILL.md")
	skillMeta, err = s.parseSkillMd(skillMdPath)
	if err != nil {
		// 如果解析失败，使用文件名作为默认值
		skillMeta = &SkillMeta{
			Name:      skillRootDir,
			SkillType: "skill",
			Version:   "1.0.0",
		}
	}

	// 如果 SKILL.md 中没有提供信息，使用默认值
	if skillMeta.Name == "" {
		skillMeta.Name = skillRootDir
	}
	if skillMeta.SkillType == "" {
		skillMeta.SkillType = "skill"
	}
	if skillMeta.Version == "" {
		skillMeta.Version = "1.0.0"
	}

	return targetDir, skillMeta, nil
}

// extractFile 解压单个文件
func (s *SysSkillService) extractFile(f *zip.File, targetDir string) error {
	// 安全检查：确保文件路径不包含 ../ 等恶意路径
	if strings.Contains(f.Name, "..") {
		return fmt.Errorf("invalid file path: %s", f.Name)
	}

	filePath := filepath.Join(targetDir, f.Name)

	// 如果是目录，创建并返回
	if f.FileInfo().IsDir() {
		return os.MkdirAll(filePath, 0755)
	}

	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	// 创建目标文件
	dst, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 打开源文件
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// 复制内容
	_, err = io.Copy(dst, src)
	return err
}

// parseSkillMd 解析 SKILL.md 提取元信息
func (s *SysSkillService) parseSkillMd(skillMdPath string) (*SkillMeta, error) {
	data, err := os.ReadFile(skillMdPath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	meta := &SkillMeta{}

	// 简单解析 YAML front matter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			yamlContent := parts[1]
			lines := strings.Split(yamlContent, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "name:") {
					meta.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				} else if strings.HasPrefix(line, "description:") {
					meta.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				} else if strings.HasPrefix(line, "type:") {
					meta.SkillType = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
				} else if strings.HasPrefix(line, "version:") {
					meta.Version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
				}
			}
		}
	}

	return meta, nil
}
