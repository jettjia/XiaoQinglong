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
	{
		Title: "pgsql",
		Content: `PostgreSQL，常被简称为 Postgres，是一款功能强大的开源关系型数据库管理系统（RDBMS）。它起源于 1986 年，由加州大学伯克利分校的计算机科学系开发，在开源社区的持续贡献下，不断发展和完善。如今，PostgreSQL 以其高度的稳定性、遵循标准的 SQL 支持以及丰富的功能特性，在全球范围内被广泛应用于各种规模的项目中。### 2.1 数据类型丰富

PostgreSQL 支持多种数据类型，除了常见的整数、浮点数、字符串、日期和时间类型外，还提供了数组、JSON、XML、几何类型等。这使得它能够处理各种复杂的数据结构，满足不同领域的需求。例如，在地理信息系统（GIS）中，可以使用几何类型存储和处理地理数据；在 Web 应用中，JSON 类型可以方便地存储和查询半结构化数据。### 2.2 高度的事务支持

PostgreSQL 支持 ACID（原子性、一致性、隔离性、持久性）事务，确保在并发环境下数据的完整性和一致性。它提供了多种事务隔离级别，如读未提交（Read Uncommitted）、读已提交（Read Committed）、可重复读（Repeatable Read）和串行化（Serializable），可以根据应用的需求选择合适的隔离级别。### 2.3 强大的扩展性

PostgreSQL 具有出色的扩展性，可以通过自定义函数、存储过程、触发器等方式扩展其功能。此外，它还支持插件机制，用户可以根据需要安装各种插件来增强数据库的功能，如全文搜索插件、空间数据插件等。### 2.4 良好的性能

PostgreSQL 在处理大量数据和高并发场景下表现出色。它采用了先进的查询优化器，能够根据数据分布和查询条件生成最优的执行计划。同时，它还支持并行查询和异步 I/O 等技术，进一步提高了查询性能。

### 2.5 多语言支持

PostgreSQL 支持多种编程语言的接口，如 Python、Java、C#、Ruby 等。这使得开发者可以方便地使用自己熟悉的编程语言与 PostgreSQL 进行交互，开发出高效、稳定的应用程序。## 三、应用场景

### 3.1 Web 应用

由于 PostgreSQL 的高性能和丰富的功能，它被广泛应用于各种 Web 应用中，如电子商务、社交网络、内容管理系统等。它可以存储和管理用户信息、商品信息、订单信息等大量数据，并提供高效的查询和处理能力。

### 3.2 数据分析和商业智能

PostgreSQL 支持复杂的查询和分析操作，如聚合查询、窗口函数、递归查询等。这些功能使得它成为数据分析和商业智能领域的理想选择，可以用于数据仓库、报表生成、数据分析等场景。

### 3.3 地理信息系统（GIS）

PostgreSQL 结合 PostGIS 插件，可以处理地理空间数据，如地图数据、地理位置信息等。它提供了丰富的地理空间函数和操作符，支持空间查询和分析，广泛应用于地理信息系统、导航系统等领域。

### 3.4 企业级应用

在企业级应用中，数据的安全性和可靠性至关重要。PostgreSQL 提供了完善的安全机制，如用户认证、授权管理、数据加密等，可以满足企业对数据安全的严格要求。同时，它的高可用性和容错能力也使得它成为企业级应用的首选数据库之一。

## 四、总结

PostgreSQL 是一款功能强大、性能优越、扩展性良好的开源关系型数据库管理系统。它具有丰富的数据类型、高度的事务支持、强大的扩展性和良好的性能，适用于各种规模和类型的项目。无论是 Web 应用、数据分析、地理信息系统还是企业级应用，PostgreSQL 都能够提供可靠的数据存储和管理解决方案。随着开源社区的不断发展和壮大，PostgreSQL 的功能和性能还将不断提升，为开发者和企业带来更多的价值。`,
		Score: 0.78,
		Tags:  []string{"数据库", "pgsql"},
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
