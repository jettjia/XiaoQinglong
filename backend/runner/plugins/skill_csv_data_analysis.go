package plugins

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// CsvDataAnalysisReportPlaceholders csv-data-analysis 报告模板的占位符
type CsvDataAnalysisReportPlaceholders struct {
	LANG                 string
	ReportTitle          string
	ReportSubtitle       string
	ExecSummary          string
	DistributionInsights string
	CorrelationInsights  string
	CategoricalInsights  string
	TimeSeriesInsights   string
	Conclusions          string
	ChartDataJSON        string
}

// ExtractCsvInsightsFromRawText 从原始文本中提取 csv-data-analysis 报告所需的数据
// 当 inner agent 直接输出 csv_analyzer 的原始结果（包含 ###CHART_DATA_JSON_START###...###CHART_DATA_JSON_END### 标记）时
// 使用此函数提取报告所需的数据
func ExtractCsvInsightsFromRawText(rawText string) *CsvDataAnalysisReportPlaceholders {
	result := &CsvDataAnalysisReportPlaceholders{
		LANG:                 "zh",
		ReportTitle:          "数据分析报告",
		ReportSubtitle:       "基于CSV数据的自动分析",
		ExecSummary:          extractExecSummaryFromRaw(rawText),
		DistributionInsights: extractDistributionInsightsFromRaw(rawText),
		CorrelationInsights:  extractCorrelationInsightsFromRaw(rawText),
		CategoricalInsights:  extractCategoricalInsightsFromRaw(rawText),
		TimeSeriesInsights:   extractTimeSeriesInsightsFromRaw(rawText),
		Conclusions:          extractConclusionsFromRaw(rawText),
	}

	// 提取图表数据
	chartData := extractChartDataFromMarkers(rawText)
	if chartData != "" {
		result.ChartDataJSON = chartData
	}

	return result
}

// extractChartDataFromMarkers 从 HTML 内容中提取 CHART_DATA_JSON
func extractChartDataFromMarkers(htmlContent string) string {
	// 查找 ###CHART_DATA_JSON_START###...###CHART_DATA_JSON_END### 标记
	startMarker := "###CHART_DATA_JSON_START###"
	endMarker := "###CHART_DATA_JSON_END###"

	logger.Infof("[extractChartDataFromMarkers] Input length=%d, searching for marker: %s", len(htmlContent), startMarker)

	startIdx := strings.Index(htmlContent, startMarker)
	if startIdx == -1 {
		// 尝试查找没有 ### 前缀的标记
		startMarker = "CHART_DATA_JSON_START"
		endMarker = "CHART_DATA_JSON_END"
		startIdx = strings.Index(htmlContent, startMarker)
		logger.Infof("[extractChartDataFromMarkers] Try without ### prefix, found at: %d", startIdx)
	}

	if startIdx == -1 {
		previewLen := 200
		if previewLen > len(htmlContent) {
			previewLen = len(htmlContent)
		}
		logger.Infof("[extractChartDataFromMarkers] No chart data markers found. Content preview: %.200s...", htmlContent[:previewLen])
		return ""
	}

	logger.Infof("[extractChartDataFromMarkers] Found start marker at index %d", startIdx)
	startIdx += len(startMarker)
	endIdx := strings.Index(htmlContent[startIdx:], endMarker)
	if endIdx == -1 {
		logger.Infof("[extractChartDataFromMarkers] Found start marker but no end marker")
		return ""
	}

	chartData := htmlContent[startIdx : startIdx+endIdx]
	chartData = strings.TrimSpace(chartData)

	previewLen := 100
	if previewLen > len(chartData) {
		previewLen = len(chartData)
	}
	logger.Infof("[extractChartDataFromMarkers] Extracted chart data, length=%d, preview: %.100s...", len(chartData), chartData[:previewLen])
	return chartData
}

// extractChartDataFromMapFields 从 map 的字符串字段中提取 CHART_DATA_JSON
// 当 data 是 map 但缺少 CHART_DATA_JSON 时调用此函数
func extractChartDataFromMapFields(data map[string]any) string {
	for _, v := range data {
		if str, ok := v.(string); ok && str != "" {
			if strings.Contains(str, "CHART_DATA_JSON_START") || strings.Contains(str, "CHART_DATA") {
				chartData := extractChartDataFromMarkers(str)
				if chartData != "" {
					return chartData
				}
			}
		}
	}
	return ""
}

// extractExecSummaryFromRaw 从原始文本中提取执行摘要
func extractExecSummaryFromRaw(rawText string) string {
	// csv_analyzer 输出的统计摘要格式包含 "==================================================" 等标记
	if strings.Contains(rawText, "==================================================") {
		lines := strings.Split(rawText, "\n")
		var summaryLines []string
		var inSummary bool
		for _, line := range lines {
			if strings.Contains(line, "==================================================") {
				if !inSummary {
					inSummary = true
					continue
				} else {
					break
				}
			}
			if inSummary && strings.TrimSpace(line) != "" {
				summaryLines = append(summaryLines, line)
			}
		}
		if len(summaryLines) > 0 {
			return strings.Join(summaryLines, "\n")
		}
	}

	// 尝试提取数据概览信息（包含 "数据集尺寸", "缺失值", "重复行" 等）
	if strings.Contains(rawText, "数据集尺寸") || strings.Contains(rawText, "数据概览") {
		overviewPatterns := []string{
			"数据集尺寸",
			"缺失值情况",
			"重复行",
			"内存占用",
		}
		var overviewLines []string
		for _, pattern := range overviewPatterns {
			idx := strings.Index(rawText, pattern)
			if idx > 0 {
				start := idx - 100
				if start < 0 {
					start = 0
				}
				end := idx + 200
				if end > len(rawText) {
					end = len(rawText)
				}
				section := rawText[start:end]
				section = cleanHTMLToText(section)
				if len(section) > 20 {
					overviewLines = append(overviewLines, section)
				}
			}
		}
		if len(overviewLines) > 0 {
			return strings.Join(overviewLines, " | ")
		}
	}

	return "数据概览完成"
}

// extractDistributionInsightsFromRaw 从原始文本中提取分布洞察
func extractDistributionInsightsFromRaw(rawText string) string {
	return extractInsightBySection(rawText, "【数值型特征统计", "【波动性与偏态重点")
}

// extractCorrelationInsightsFromRaw 从原始文本中提取相关性洞察
func extractCorrelationInsightsFromRaw(rawText string) string {
	return extractInsightBySection(rawText, "【核心相关性", "【散点图")
}

// extractCategoricalInsightsFromRaw 从原始文本中提取分类洞察
func extractCategoricalInsightsFromRaw(rawText string) string {
	return extractInsightBySection(rawText, "【分类型特征摘要", "【分类维度切片表现")
}

// extractTimeSeriesInsightsFromRaw 从原始文本中提取时序洞察
func extractTimeSeriesInsightsFromRaw(rawText string) string {
	return extractInsightBySection(rawText, "【时间序列", "==================================================")
}

// extractConclusionsFromRaw 从原始文本中提取结论
func extractConclusionsFromRaw(rawText string) string {
	// 结论通常在最后部分
	sections := strings.Split(rawText, "==================================================")
	if len(sections) > 2 {
		// 取最后一部分作为结论
		conclusion := sections[len(sections)-1]
		conclusion = strings.TrimSpace(conclusion)
		if len(conclusion) > 20 {
			return conclusion
		}
	}
	return "分析完成"
}

// extractInsightBySection 根据章节标记提取洞察内容
func extractInsightBySection(rawText string, startMarker string, endMarker string) string {
	startIdx := strings.Index(rawText, startMarker)
	if startIdx == -1 {
		return "无数据"
	}

	endIdx := strings.Index(rawText[startIdx:], endMarker)
	if endIdx == -1 {
		// 没有结束标记，取到下一个 === 标记之前
		nextSection := strings.Index(rawText[startIdx+len(startMarker):], "==================================================")
		if nextSection > 0 {
			endIdx = nextSection
		} else {
			return "无数据"
		}
	}

	section := rawText[startIdx : startIdx+endIdx]
	section = cleanHTMLToText(section)
	if len(section) < 10 {
		return "无数据"
	}
	return section
}

// cleanHTMLToText 清理 HTML 标签，提取纯文本
func cleanHTMLToText(html string) string {
	// 移除 HTML 注释
	html = strings.ReplaceAll(html, "<!--", "")
	html = strings.ReplaceAll(html, "-->", "")

	// 替换常见的 HTML 标签为空格
	html = strings.ReplaceAll(html, "<br>", "\n")
	html = strings.ReplaceAll(html, "<br/>", "\n")
	html = strings.ReplaceAll(html, "<br />", "\n")
	html = strings.ReplaceAll(html, "</p>", "\n")
	html = strings.ReplaceAll(html, "</div>", "\n")
	html = strings.ReplaceAll(html, "</li>", "\n")
	html = strings.ReplaceAll(html, "</tr>", "\n")

	// 移除所有 HTML 标签
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	text := result.String()

	// 清理多余空白
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, " ")
}

// ToMap 将 CsvDataAnalysisReportPlaceholders 转换为 map[string]any
func (p *CsvDataAnalysisReportPlaceholders) ToMap() map[string]any {
	data := map[string]any{
		"LANG":                  p.LANG,
		"REPORT_TITLE":          p.ReportTitle,
		"REPORT_SUBTITLE":       p.ReportSubtitle,
		"EXEC_SUMMARY":          p.ExecSummary,
		"DISTRIBUTION_INSIGHTS": p.DistributionInsights,
		"CORRELATION_INSIGHTS":  p.CorrelationInsights,
		"CATEGORICAL_INSIGHTS":  p.CategoricalInsights,
		"TIME_SERIES_INSIGHTS":  p.TimeSeriesInsights,
		"CONCLUSIONS":           p.Conclusions,
	}

	if p.ChartDataJSON != "" {
		data["CHART_DATA_JSON"] = p.ChartDataJSON
	}

	return data
}

// postProcessSkillOutput 后处理 skill 输出，检测并处理未填充的模板
// 当 inner agent 没有正确调用 htmlInterpreterTool 时，由 outer agent 后处理
func (r *SkillRunner) postProcessSkillOutput(content string, tools []tool.BaseTool) string {
	// 调试：记录原始内容
	logger.Infof("[Skill] PostProcess: input content length=%d, preview=%.200s...", len(content), content[:min(200, len(content))])

	// 检查是否是 csv_analyzer 的原始 JSON 输出（包含 ###CHART_DATA_JSON_START### 标记）
	if strings.Contains(content, "###CHART_DATA_JSON_START###") {
		logger.Infof("[Skill] PostProcess: Detected csv_analyzer raw output with chart data markers, processing...")
		return r.processCsvAnalyzerOutput(content, tools)
	}

	// 检查是否包含未填充的模板占位符
	if !strings.Contains(content, "{{") {
		// 如果内容不包含模板占位符，检查是否是报告 URL
		// 报告 URL 格式：/uploads/{sessionID}/reports/report_xxx.html
		if strings.Contains(content, "/uploads/") && strings.Contains(content, "/reports/report_") {
			logger.Infof("[Skill] PostProcess: Detected report URL: %s", content)
			// 读取实际的 HTML 文件并检查是否有未填充的占位符
			htmlContent := r.readReportHTMLFile(content)
			if htmlContent != "" && strings.Contains(htmlContent, "{{CHART_DATA_JSON}}") {
				logger.Infof("[Skill] PostProcess: Report HTML has unfilled {{CHART_DATA_JSON}}, attempting to fix")
				// 尝试提取 chart data 并注入
				chartData := extractChartDataFromMarkers(htmlContent)
				if chartData != "" {
					htmlContent = strings.ReplaceAll(htmlContent, "{{CHART_DATA_JSON}}", chartData)
					r.saveReportHTMLFile(content, htmlContent)
					logger.Infof("[Skill] PostProcess: Injected chart data into report HTML")
				}
			}
			return content
		}
		logger.Infof("[Skill] PostProcess: content does not contain {{ or report URL, returning as-is")
		return content
	}

	// 检查是否是 HTML 模板（包含 DOCTYPE 或 <html>）
	if !strings.Contains(content, "<!DOCTYPE") && !strings.Contains(content, "<html") {
		return content
	}

	logger.Infof("[Skill] PostProcess: Detected unfilled HTML template, attempting to process")

	// 找到 htmlInterpreterTool
	var htmlTool *htmlInterpreterTool
	for _, t := range tools {
		if info, _ := t.Info(context.Background()); info != nil && info.Name == "html_interpreter" {
			if hit, ok := t.(*htmlInterpreterTool); ok {
				htmlTool = hit
				break
			}
		}
	}
	if htmlTool == nil {
		logger.Infof("[Skill] PostProcess: html_interpreter tool not found")
		return content
	}

	// 尝试提取模板路径和数据
	templatePath, data := r.extractTemplateData(content)
	if templatePath == "" || data == nil {
		logger.Infof("[Skill] PostProcess: Could not extract template path or data")
		return content
	}

	// 调用 html_interpreter
	args := map[string]any{
		"template_path": templatePath,
		"data":          data,
	}
	argsJSON, _ := json.Marshal(args)
	result, err := htmlTool.InvokableRun(context.Background(), string(argsJSON))
	if err != nil {
		logger.Infof("[Skill] PostProcess: html_interpreter failed: %v", err)
		return content
	}

	logger.Infof("[Skill] PostProcess: html_interpreter succeeded, result length=%d", len(result))

	// 检查结果中是否还包含未替换的 {{CHART_DATA_JSON}}
	if strings.Contains(result, "{{CHART_DATA_JSON}}") {
		logger.Infof("[Skill] PostProcess: Result still contains {{CHART_DATA_JSON}} placeholder, attempting to inject chart data")
		// 从 content 中提取图表数据
		chartData := extractChartDataFromMarkers(content)
		if chartData != "" {
			result = strings.ReplaceAll(result, "{{CHART_DATA_JSON}}", chartData)
			logger.Infof("[Skill] PostProcess: Re-extracted and injected chart data, new length=%d", len(result))
		} else {
			logger.Infof("[Skill] PostProcess: Failed to extract chart data from content")
		}
	}

	return result
}

// processCsvAnalyzerOutput 处理 csv_analyzer 的原始输出，调用 htmlInterpreterTool 生成报告
func (r *SkillRunner) processCsvAnalyzerOutput(content string, tools []tool.BaseTool) string {
	// 找到 htmlInterpreterTool
	var htmlTool *htmlInterpreterTool
	for _, t := range tools {
		if info, _ := t.Info(context.Background()); info != nil && info.Name == "html_interpreter" {
			if hit, ok := t.(*htmlInterpreterTool); ok {
				htmlTool = hit
				break
			}
		}
	}
	if htmlTool == nil {
		logger.Infof("[Skill] PostProcess: html_interpreter tool not found")
		return content
	}

	// 使用 ExtractCsvInsightsFromRawText 提取数据
	insights := ExtractCsvInsightsFromRawText(content)
	data := insights.ToMap()

	// 调试日志：检查提取的数据
	logger.Infof("[Skill] PostProcess: ChartDataJSON present in map: %v", data["CHART_DATA_JSON"] != nil && data["CHART_DATA_JSON"] != "")
	if data["CHART_DATA_JSON"] != nil {
		logger.Infof("[Skill] PostProcess: ChartDataJSON length: %d, preview: %.100s...",
			len(data["CHART_DATA_JSON"].(string)), data["CHART_DATA_JSON"].(string))
	}
	logger.Infof("[Skill] PostProcess: Extracted chart data length=%d, execSummary length=%d",
		len(insights.ChartDataJSON), len(insights.ExecSummary))

	// 调用 html_interpreter
	templatePath := "templates/report_template.html"
	args := map[string]any{
		"template_path": templatePath,
		"data":          data,
	}
	argsJSON, _ := json.Marshal(args)
	result, err := htmlTool.InvokableRun(context.Background(), string(argsJSON))
	if err != nil {
		logger.Infof("[Skill] PostProcess: html_interpreter failed: %v", err)
		return content
	}

	logger.Infof("[Skill] PostProcess: html_interpreter succeeded, result length=%d", len(result))
	return result
}
