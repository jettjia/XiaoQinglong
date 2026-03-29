package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// Mock Knowledge Base Retrieval Service

// KnowledgeEntry represents a knowledge base entry
type KnowledgeEntry struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Score   float64  `json:"score"`
	Tags    []string `json:"tags,omitempty"`
}

// KnowledgeDatabase mock knowledge database
var knowledgeDB = []KnowledgeEntry{
	{
		Title:   "远程办公政策",
		Content: "员工入职满3个月后可申请远程办公，需部门经理批准。远程办公期间需保持在线状态，定期提交工作汇报。",
		Score:   0.95,
		Tags:    []string{"HR", "政策", "远程办公"},
	},
	{
		Title:   "密码重置指南",
		Content: "访问自助门户 https://reset.company.com 重置密码，或联系 IT 支持邮箱 it-support@company.com。",
		Score:   0.92,
		Tags:    []string{"IT", "密码", "自助服务"},
	},
	{
		Title:   "请假政策",
		Content: "员工每年享有15天带薪年假，需提前3天通过 OA 系统申请。病假需提供医院证明，事假每月不超过3天。",
		Score:   0.90,
		Tags:    []string{"HR", "请假", "假期"},
	},
	{
		Title:   "报销流程",
		Content: "通过费控系统提交报销申请，平均处理时间3个工作日。单笔超过5000元需部门负责人审批。",
		Score:   0.88,
		Tags:    []string{"财务", "报销", "流程"},
	},
	{
		Title:   "IT 支持联系",
		Content: "联系 IT 支持邮箱 it-support@company.com 或拨打内部分机 8001，服务时间周一至周五 9:00-18:00。",
		Score:   0.85,
		Tags:    []string{"IT", "支持", "联系"},
	},
	{
		Title:   "会议室预订",
		Content: "通过企业微信「会议室预订」小程序或 OA 系统预订会议室。会议开始前5分钟自动释放未签到会议室。",
		Score:   0.82,
		Tags:    []string{"行政", "会议", "预订"},
	},
	{
		Title:   "入职流程",
		Content: "新员工入职第一天到 HR 报到，领取工牌和电脑。第二三天参加新员工培训，一周内完成系统账号开通。",
		Score:   0.80,
		Tags:    []string{"HR", "入职", "新员工"},
	},
	{
		Title:   "加班调休政策",
		Content: "工作日加班可申请调休，节假日前后加班按国家规定支付加班费。调休需在产生后30天内使用。",
		Score:   0.78,
		Tags:    []string{"HR", "加班", "调休"},
	},
}

// RetrievalRequest represents the retrieval request
type RetrievalRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

// simpleSearch performs a simple keyword-based search
func simpleSearch(query string, topK int) []KnowledgeEntry {
	query = strings.ToLower(query)
	results := []KnowledgeEntry{}

	for _, entry := range knowledgeDB {
		// Simple scoring based on keyword matches
		score := 0.0
		keywords := strings.Fields(query)

		titleLower := strings.ToLower(entry.Title)
		contentLower := strings.ToLower(entry.Content)

		for _, keyword := range keywords {
			if strings.Contains(titleLower, keyword) {
				score += 0.4
			}
			if strings.Contains(contentLower, keyword) {
				score += 0.2
			}
			for _, tag := range entry.Tags {
				if strings.Contains(strings.ToLower(tag), keyword) {
					score += 0.1
				}
			}
		}

		if score > 0 {
			entry.Score = score
			results = append(results, entry)
		}
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Limit results
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results
}

func main() {
	// Retrieval endpoint
	http.HandleFunc("/retrieve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req RetrievalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Set default topK if not specified
		if req.TopK == 0 {
			req.TopK = 5
		}

		// Search knowledge base
		results := simpleSearch(req.Query, req.TopK)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.Println("Mock KB Service starting on :28083")
	log.Println("Available endpoints:")
	log.Println("  - POST /retrieve  (main retrieval endpoint)")
	log.Println("  - GET  /health    (health check)")
	log.Println("")
	log.Println("Example curl command:")
	log.Println("  curl -X POST http://localhost:28083/retrieve -d '{\"query\": \"远程办公\", \"top_k\": 3}'")
	log.Fatal(http.ListenAndServe(":28083", nil))
}
