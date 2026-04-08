package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// ========== PptxInterpreterTool ==========

// pptxInterpreterTool PPT生成解释器工具
// 接收slide配置，生成JavaScript代码，执行node命令生成PPT，并保存到reports目录
type pptxInterpreterTool struct {
	workDir    string // 工作目录，用于存储临时JS文件
	reportsDir string // PPT报告输出目录
	baseURL    string // PPT访问的基础URL
}

// SlideConfig 幻灯片配置
type SlideConfig struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	Bg          string `json:"bg"`
	TitleColor  string `json:"title_color"`
	ContentColor string `json:"content_color"`
	TitleSize   int    `json:"title_size"`
	ContentSize int    `json:"content_size"`
}

// NewPptxInterpreterTool 创建PPT解释器工具
func NewPptxInterpreterTool(workDir, reportsDir, reportsURL string) *pptxInterpreterTool {
	os.MkdirAll(reportsDir, 0755)
	return &pptxInterpreterTool{
		workDir:    workDir,
		reportsDir: reportsDir,
		baseURL:    reportsURL,
	}
}

func (t *pptxInterpreterTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "pptx_interpreter",
		Desc: "根据配置生成 PowerPoint PPT 文件。接收slides数组配置，生成专业的PPT并保存到报告目录，返回可访问的URL。使用方法：1) title 传入PPT标题；2) author 传入作者；3) filename 传入保存的文件名；4) slides 传入幻灯片数组，每个slide包含 title、content、bg(背景色)、title_color、content_color 等字段。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title":     {Type: schema.String, Desc: "PPT标题", Required: true},
			"author":    {Type: schema.String, Desc: "作者", Required: false},
			"filename":  {Type: schema.String, Desc: "保存的文件名，如 presentation.pptx", Required: false},
			"slides":    {Type: schema.Object, Desc: "幻灯片数组，每个slide是对象，包含 title(标题)、content(内容)、bg(背景色)、title_color、content_size 等字段", Required: true},
		}),
	}, nil
}

func (t *pptxInterpreterTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	type req struct {
		Title    string        `json:"title"`
		Author   string        `json:"author"`
		Filename string        `json:"filename"`
		Slides   []SlideConfig `json:"slides"`
	}

	var r req
	if err := json.Unmarshal([]byte(argumentsInJSON), &r); err != nil {
		return "", fmt.Errorf("parse arguments failed: %w", err)
	}

	if len(r.Slides) == 0 {
		return "", fmt.Errorf("slides is required and cannot be empty")
	}

	// 设置默认值
	if r.Author == "" {
		r.Author = "AI Assistant"
	}
	if r.Filename == "" {
		r.Filename = fmt.Sprintf("presentation_%d.pptx", time.Now().UnixNano())
	}
	if !strings.HasSuffix(r.Filename, ".pptx") {
		r.Filename += ".pptx"
	}

	// 生成JavaScript代码
	jsCode := t.generatePptxJs(r.Title, r.Author, r.Slides)

	// 写入临时JS文件
	tmpJs := filepath.Join(t.workDir, fmt.Sprintf("pptx_gen_%d.js", time.Now().UnixNano()))
	if err := os.WriteFile(tmpJs, []byte(jsCode), 0644); err != nil {
		return "", fmt.Errorf("write js file failed: %w", err)
	}
	defer os.Remove(tmpJs)

	// 检查node是否可用
	if _, err := exec.LookPath("node"); err != nil {
		// node不可用，尝试在sandbox中执行
		return t.execInDocker(ctx, tmpJs, r.Filename)
	}

	// 本地执行
	return t.execLocally(ctx, tmpJs, r.Filename)
}

// execLocally 本地执行node命令生成PPT
func (t *pptxInterpreterTool) execLocally(ctx context.Context, jsFile, filename string) (string, error) {
	// 创建临时输出目录
	tmpOutput := filepath.Join(t.workDir, "pptx_output")
	os.MkdirAll(tmpOutput, 0755)

	// 修改JS代码中的输出路径
	outputPath := filepath.Join(tmpOutput, filename)

	// 读取JS文件内容
	jsContent, err := os.ReadFile(jsFile)
	if err != nil {
		return "", fmt.Errorf("read js file failed: %w", err)
	}

	// 修改输出路径
	jsContentStr := string(jsContent)
	jsContentStr = strings.ReplaceAll(jsContentStr, "{{OUTPUT_PATH}}", outputPath)

	// 写入修改后的JS文件
	tmpJsModified := filepath.Join(t.workDir, fmt.Sprintf("pptx_gen_modified_%d.js", time.Now().UnixNano()))
	if err := os.WriteFile(tmpJsModified, []byte(jsContentStr), 0644); err != nil {
		return "", fmt.Errorf("write modified js file failed: %w", err)
	}
	defer os.Remove(tmpJsModified)

	// 安装pptxgenjs（如果需要）
	installCmd := exec.Command("npm", "list", "pptxgenjs")
	installCmd.Dir = t.workDir
	if err := installCmd.Run(); err != nil {
		logger.Infof("[pptxInterpreterTool] pptxgenjs not found, installing...")
		installCmd = exec.Command("npm", "install", "pptxgenjs", "--silent")
		installCmd.Dir = t.workDir
		if err := installCmd.Run(); err != nil {
			logger.Warnf("[pptxInterpreterTool] install pptxgenjs failed: %v", err)
		}
	}

	// 执行node命令
	cmd := exec.Command("node", tmpJsModified)
	cmd.Dir = t.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("[pptxInterpreterTool] node exec failed: %v, output: %s", err, string(output))
		return "", fmt.Errorf("node exec failed: %s", string(output))
	}

	logger.Infof("[pptxInterpreterTool] node output: %s", string(output))

	// 检查输出文件是否存在
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("pptx file not generated: %s", outputPath)
	}

	// 复制到reports目录
	finalPath := filepath.Join(t.reportsDir, filename)
	if err := copyFile(outputPath, finalPath); err != nil {
		return "", fmt.Errorf("copy pptx to reports failed: %w", err)
	}

	// 清理临时目录
	os.RemoveAll(tmpOutput)

	// 返回PPT URL
	pptxURL := t.baseURL + "/" + filename
	logger.Infof("[pptxInterpreterTool] PPT generated successfully: %s", pptxURL)
	return pptxURL, nil
}

// execInDocker 在Docker容器中执行node命令生成PPT
func (t *pptxInterpreterTool) execInDocker(ctx context.Context, jsFile, filename string) (string, error) {
	dockerBin := "docker"
	if _, err := exec.LookPath(dockerBin); err != nil {
		return "", fmt.Errorf("docker not found: %w", err)
	}

	// 创建临时输出目录（宿主机端）
	tmpOutput := filepath.Join(t.workDir, "pptx_output")
	os.MkdirAll(tmpOutput, 0755)
	defer os.RemoveAll(tmpOutput)

	outputPath := filepath.Join(tmpOutput, filename)

	// 读取并修改JS文件
	jsContent, err := os.ReadFile(jsFile)
	if err != nil {
		return "", fmt.Errorf("read js file failed: %w", err)
	}
	jsContentStr := strings.ReplaceAll(string(jsContent), "{{OUTPUT_PATH}}", "/workspace/pptx_output/"+filename)
	tmpJsModified := filepath.Join(t.workDir, fmt.Sprintf("pptx_gen_modified_%d.js", time.Now().UnixNano()))
	if err := os.WriteFile(tmpJsModified, []byte(jsContentStr), 0644); err != nil {
		return "", fmt.Errorf("write modified js file failed: %w", err)
	}
	defer os.Remove(tmpJsModified)

	containerName := fmt.Sprintf("pptx-gen-%d", time.Now().UnixNano())
	image := "node:18-alpine"

	// 构建docker命令：在node容器中执行JS，输出到挂载的目录
	cmd := exec.Command(dockerBin, "run", "--rm", "--name", containerName,
		"-v", fmt.Sprintf("%s:/workspace", t.workDir),
		"-w", "/workspace",
		image, "sh", "-c",
		fmt.Sprintf("npm install pptxgenjs --silent 2>/dev/null && node /workspace/%s", filepath.Base(tmpJsModified)))

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("[pptxInterpreterTool] docker exec failed: %v, output: %s", err, string(output))
		return "", fmt.Errorf("docker exec failed: %s", string(output))
	}

	logger.Infof("[pptxInterpreterTool] docker output: %s", string(output))

	// 检查输出文件
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("pptx file not generated in docker: %s", outputPath)
	}

	// 复制到reports目录
	finalPath := filepath.Join(t.reportsDir, filename)
	if err := copyFile(outputPath, finalPath); err != nil {
		return "", fmt.Errorf("copy pptx to reports failed: %w", err)
	}

	// 返回PPT URL
	pptxURL := t.baseURL + "/" + filename
	logger.Infof("[pptxInterpreterTool] PPT generated via docker: %s", pptxURL)
	return pptxURL, nil
}

// generatePptxJs 生成pptxgenjs的JavaScript代码
func (t *pptxInterpreterTool) generatePptxJs(title, author string, slides []SlideConfig) string {
	var slideCode strings.Builder

	for i, slide := range slides {
		bg := slide.Bg
		if bg == "" {
			bg = "FFFFFF"
		}
		titleColor := slide.TitleColor
		if titleColor == "" {
			titleColor = "1E2761"
		}
		contentColor := slide.ContentColor
		if contentColor == "" {
			contentColor = "363636"
		}
		titleSize := slide.TitleSize
		if titleSize == 0 {
			titleSize = 36
		}
		contentSize := slide.ContentSize
		if contentSize == 0 {
			contentSize = 18
		}

		// 转义内容中的特殊字符
		content := strings.ReplaceAll(slide.Content, `"`, `\"`)
		content = strings.ReplaceAll(content, "`", `\"`)

		slideCode.WriteString(fmt.Sprintf(`
	const slide%d = pres.addSlide();
	slide%d.background = { color: "%s" };
	slide%d.addText("%s", {
		x: 0.5, y: 0.5, w: 9, h: 1,
		fontSize: %d,
		color: "%s",
		bold: true
	});
	slide%d.addText("%s", {
		x: 0.5, y: 1.8, w: 9, h: 3.5,
		fontSize: %d,
		color: "%s",
		valign: "top"
	});`, i, i, bg, i, slide.Title, titleSize, titleColor, i, content, contentSize, contentColor))
	}

	js := fmt.Sprintf(`const pptxgen = require("pptxgenjs");
const pres = new pptxgen();
pres.layout = "LAYOUT_16x9";
pres.title = "%s";
pres.author = "%s";
%s
pres.writeFile({ fileName: "{{OUTPUT_PATH}}" })
	.then(() => console.log("Created:", "{{OUTPUT_PATH}}"))
	.catch(err => console.error("Error:", err));
`, title, author, slideCode.String())

	return js
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
