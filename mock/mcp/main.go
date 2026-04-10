package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	mcpServer := server.NewMCPServer("mock-mcp-server", mcp.LATEST_PROTOCOL_VERSION)

	// 天气查询 tool
	mcpServer.AddTool(WeatherTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := request.GetArguments()
		city, ok := params["city"].(string)
		if !ok || city == "" {
			return nil, fmt.Errorf("city is required")
		}

		baseURL := "https://restapi.amap.com/v3/weather/weatherInfo"
		queryParams := url.Values{}
		queryParams.Set("city", city)
		apiKey := strings.TrimSpace(os.Getenv("AMAP_API_KEY"))
		if apiKey == "" {
			extensions := "base"
			if v, ok := params["extensions"].(string); ok && strings.TrimSpace(v) != "" {
				extensions = v
			}
			return mcp.NewToolResultText(fmt.Sprintf("{\"mock\":true,\"city\":%q,\"extensions\":%q,\"weather\":\"sunny\",\"temp\":\"20\",\"note\":\"AMAP_API_KEY not set\"}", city, extensions)), nil
		}
		queryParams.Set("key", apiKey)

		if extensions, ok := params["extensions"].(string); ok {
			queryParams.Set("extensions", extensions)
		} else {
			queryParams.Set("extensions", "base")
		}
		queryParams.Set("output", "JSON")

		fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

		req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return mcp.NewToolResultText(string(body)), nil
	})

	// 订单查询 tool
	mcpServer.AddTool(GetOrderTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_ = ctx
		params := request.GetArguments()
		orderID, ok := params["order_id"].(string)
		if !ok || orderID == "" {
			return nil, fmt.Errorf("order_id is required")
		}

		order := map[string]any{
			"order_id": orderID,
			"status":   "shipped",
			"total":    199.99,
			"items":    []string{"商品A", "商品B"},
		}
		raw, _ := json.Marshal(order)
		return mcp.NewToolResultText(string(raw)), nil
	})

	// 销售订单列表 tool
	mcpServer.AddTool(SalesOrdersTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_ = ctx
		params := request.GetArguments()
		company, _ := params["company"].(string)
		company = strings.TrimSpace(company)
		if company == "" {
			return nil, fmt.Errorf("company is required")
		}
		month, _ := params["month"].(string)
		month = strings.TrimSpace(month)
		if month == "" {
			month = time.Now().Format("2006-01")
		}

		seed := int64(0)
		for _, r := range []rune(company + "|" + month) {
			seed += int64(r)
		}
		rng := rand.New(rand.NewSource(seed))

		type order struct {
			OrderNo  string  `json:"order_no"`
			Date     string  `json:"date"`
			Customer string  `json:"customer"`
			Amount   float64 `json:"amount"`
			Status   string  `json:"status"`
		}
		customers := []string{"上海华东商贸", "北京云启科技", "深圳海风电商", "杭州星云网络", "成都西岭商行", "南京智联贸易", "苏州同盛工贸"}
		statuses := []string{"已签收", "已发货", "待发货", "已取消"}
		orderCount := 8 + rng.Intn(8)
		orders := make([]order, 0, orderCount)
		for i := 0; i < orderCount; i++ {
			day := 1 + rng.Intn(28)
			orders = append(orders, order{
				OrderNo:  fmt.Sprintf("SO-%s-%04d", strings.ReplaceAll(month, "-", ""), 1000+i),
				Date:     fmt.Sprintf("%s-%02d", month, day),
				Customer: customers[rng.Intn(len(customers))],
				Amount:   float64(5000+rng.Intn(200000)) + rng.Float64(),
				Status:   statuses[rng.Intn(len(statuses))],
			})
		}

		outObj := map[string]any{
			"company":  company,
			"month":    month,
			"currency": "CNY",
			"orders":   orders,
		}
		raw, _ := json.Marshal(outObj)
		return mcp.NewToolResultText(string(raw)), nil
	})

	// 支持 --stdio 参数切换到 stdio 模式
	for _, arg := range os.Args {
		if arg == "--stdio" || arg == "stdio" {
			// stdio 模式
			stdioSrv := server.NewStdioServer(mcpServer)
			err := stdioSrv.Listen(context.Background(), os.Stdin, os.Stdout)
			if err != nil {
				panic(err)
			}
			return
		}
	}

	// 默认 SSE 模式
	err := server.NewSSEServer(mcpServer).Start("localhost:28082")
	if err != nil {
		panic(err)
	}
}

func WeatherTool() mcp.Tool {
	tool := mcp.NewTool("get_weather",
		mcp.WithDescription("获取指定城市的天气信息"),
		mcp.WithString("city", mcp.Required(), mcp.Description("城市名称")),
		mcp.WithString("extensions",
			mcp.Required(),
			mcp.Enum("base", "all"),
			mcp.Description("返回数据类型，base为实况天气，all为预报天气"),
			mcp.DefaultString("base"),
		),
	)
	return tool
}

func GetOrderTool() mcp.Tool {
	tool := mcp.NewTool("get_order",
		mcp.WithDescription("根据订单号获取订单详情"),
		mcp.WithString("order_id", mcp.Required(), mcp.Description("订单号")),
	)
	return tool
}

func SalesOrdersTool() mcp.Tool {
	tool := mcp.NewTool("get_sales_orders",
		mcp.WithDescription("返回某公司的销售订单数据（模拟）"),
		mcp.WithString("company", mcp.Required(), mcp.Description("公司名称")),
		mcp.WithString("month", mcp.Description("月份，格式 YYYY-MM")),
	)
	return tool
}
