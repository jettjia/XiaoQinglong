package main

import (
	"context"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/a2a/extension/eino"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	hertzServer "github.com/cloudwego/hertz/pkg/app/server"
)

func main() {
	ctx := context.Background()

	hostPorts := strings.TrimSpace(os.Getenv("A2A_HOSTPORTS"))
	if hostPorts == "" {
		port := strings.TrimSpace(os.Getenv("A2A_PORT"))
		if port == "" {
			port = strings.TrimSpace(os.Getenv("PORT"))
		}
		if port == "" {
			port = "28080"
		}
		if strings.Contains(port, ":") {
			hostPorts = port
		} else {
			hostPorts = ":" + port
		}
	}

	h := hertzServer.Default(hertzServer.WithHostPorts(hostPorts))
	r, err := jsonrpc.NewRegistrar(ctx, &jsonrpc.ServerConfig{
		Router:      h,
		HandlerPath: "/a2a",
	})
	if err != nil {
		panic(err)
	}

	agent := mockAgent{}

	err = eino.RegisterServerHandlers(ctx, agent, &eino.ServerConfig{
		Registrar: r,
	})
	if err != nil {
		panic(err)
	}
	err = h.Run()
	if err != nil {
		panic(err)
	}
}

type mockAgent struct{}

func (mockAgent) Name(context.Context) string { return "payment_agent" }
func (mockAgent) Description(context.Context) string {
	return "支付 agent：处理支付、退款、订单相关问题"
}
func (mockAgent) Run(ctx context.Context, input *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		query := ""
		if len(input.Messages) > 0 {
			lastMsg := input.Messages[len(input.Messages)-1]
			if lastMsg.Role == schema.User {
				query = lastMsg.Content
			}
		}

		response := handlePaymentQuery(query)
		msg := schema.AssistantMessage(response, nil)
		gen.Send(adk.EventFromMessage(msg, nil, schema.Assistant, ""))
		gen.Close()
	}()
	return iter
}

func handlePaymentQuery(query string) string {
	return `支付处理完成。

订单号：ORD-20240315-001
支付金额：¥199.99
支付方式：微信支付
支付状态：成功

如需退款或取消，请回复"退款"或"取消订单"。`
}
