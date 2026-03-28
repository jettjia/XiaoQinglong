# eino-runner 请求示例

## 完整请求示例

以下是一个包含所有字段的完整请求示例：

```json
{
  "prompt": "你是一个专业的智能客服助手，擅长解答用户关于订单、物流、产品等问题。你的回复要友好、专业、有耐心。",
  "models": {
    "default": {
      "provider": "openai",
      "name": "gpt-4o",
      "api_base": "https://api.openai.com/v1",
      "api_key": "sk-xxxx",
      "temperature": 0.7,
      "max_tokens": 2000,
      "top_p": 0.9
    },
    "rewrite": {
      "provider": "openai",
      "name": "gpt-4o-mini",
      "api_key": "sk-xxxx",
      "temperature": 0.3,
      "max_tokens": 500
    },
    "skill": {
      "provider": "openai",
      "name": "gpt-4o",
      "api_key": "sk-xxxx",
      "temperature": 0.7,
      "max_tokens": 4000
    },
    "summarize": {
      "provider": "anthropic",
      "name": "claude-haiku-4-20250501",
      "api_key": "sk-ant-xxxx",
      "temperature": 0.5,
      "max_tokens": 1000
    }
  },
  "messages": [
    {
      "role": "system",
      "content": "你是客服助手"
    },
    {
      "role": "user",
      "content": "你好，我想查一下我的订单"
    },
    {
      "role": "assistant",
      "content": "您好！很高兴为您服务。请问您的订单号是多少？或者您可以提供下单时使用的手机号/邮箱，我来帮您查询。"
    },
    {
      "role": "user",
      "content": "订单号是 20240315001"
    }
  ],
  "context": {
    "session_id": "sess_abc123def456",
    "user_id": "user_001",
    "channel_id": "feishu",
    "variables": {
      "last_query_time": "2024-03-15T10:30:00Z",
      "user_preference": "express_shipping",
      "cart_items_count": 3
    },
    "trace_id": "uuid-123-456",
    "parent_span_id": "span-789"
  },
  "knowledge": [
    {
      "id": "kb_001",
      "name": "订单查询政策",
      "content": "订单查询说明：用户可以通过订单号、手机号、邮箱查询订单。订单状态包括：待付款、已付款、已发货、已完成、已取消。",
      "score": 0.92,
      "metadata": {
            "source": "internal_wiki",
            "url": "http://wiki.company.com/products",
            "last_updated": "2024-01-01"
      }
    },
    {
      "id": "kb_002",
      "name": "物流配送政策",
      "content": "配送政策：普通快递2-3天送达，次日达当天下午6点前下单当天发货，急速达1小时送达。",
      "score": 0.85
    }
  ],
  "skills": [
    {
      "id": "skill_order_query",
      "name": "订单查询",
      "description": "帮助用户查询订单状态、订单详情",
      "instruction": "你是一个订单查询专家，可以通过订单号查询订单的详细信息，包括订单状态、商品信息、物流信息等。",
      "scope": "chat",
      "trigger": "auto",
      "runtime": "python3.10",
      "dependencies": ["boto3"],
    },
    {
      "id": "skill_logistics",
      "name": "物流查询",
      "description": "查询快递物流信息",
      "instruction": "你是一个物流查询专家，可以根据快递单号查询物流轨迹。",
      "scope": "chat",
      "trigger": "auto",
      "entry_script": "main.py",
      "file_path": "./skills/logistics"
    }
  ],
  "mcps": [
    {
      "name": "order_system",
      "command": "npx",
      "args": ["-y", "@myorg/order-mcp-server"],
      "env": {
        "API_ENDPOINT": "https://api.example.com/orders"
      }
    },
    {
      "name": "product_catalog",
      "command": "python",
      "args": ["./mcp/product_server.py"],
      "env": {
        "DB_HOST": "localhost",
        "DB_PORT": "5432"
      }
    }
  ],
  "a2a": [
        {
            "name": "payment_agent",
            "endpoint": "http://a2a-server:8000/agent/payment",
            "headers": {
                "Authorization": "Bearer sk-test-xxx"
            }
        },
        {
            "name": "refund_agent",
            "endpoint": "http://a2a-server:8000/agent/refund",
            "headers": {
                "Authorization": "Bearer sk-test-xxx"
            }
        }
    ],
  "tools": [
    {
      "type": "http",
      "name": "get_order_detail",
      "description": "根据订单号获取订单详情",
      "endpoint": "https://api.example.com/v1/orders/{order_id}",
      "method": "GET",
      "headers": {
        "Authorization": "Bearer {api_token}",
        "Content-Type": "application/json"
      },
      "risk_level": "medium"
    },
    {
      "type": "http",
      "name": "get_express_info",
      "description": "查询快递物流信息",
      "endpoint": "https://api.example.com/v1/logistics/{tracking_number}",
      "method": "GET",
      "headers": {
        "Authorization": "Bearer {api_token}"
      },
      "risk_level": "low"
    }
  ],
  "internal_agents": [
    {
      "id": "planner",
      "name": "规划Agent",
      "prompt": "你是一个任务规划Agent。当用户提出复杂问题时，你需要将问题分解为多个步骤，并确定每个步骤应该使用哪个技能或工具。",
      "model": {
        "provider": "anthropic",
        "name": "claude-sonnet-4-6",
        "api_key": "sk-ant-xxxx"
      }
    },
    {
      "id": "summarizer",
      "name": "总结Agent",
      "prompt": "你是一个总结Agent，负责将多轮对话和工具调用结果整理成简洁、清晰的回复。",
      "model": {
        "provider": "openai",
        "name": "gpt-4o-mini",
        "api_key": "sk-xxxx"
      }
    }
  ],
  "options": {
    "temperature": 0.7,
    "max_tokens": 2000,
    "stream": false,
    "top_p": 0.9,
    "stop": null,
    "timeout_ms": 60000,
    "approval_policy": {
        "enabled": true,
        "risk_threshold": "medium", // medium及以上需要人工审批
        "auto_approve_tools": ["get_product_info"] // 白名单
    },
    "retry": {
      "max_attempts": 3,
      "initial_delay_ms": 1000,
      "max_delay_ms": 10000,
      "backoff_multiplier": 2.0,
      "retryable_errors": ["timeout", "rate_limit", "server_error"]
    },
    "response_format": {
      "type": "a2ui",
      "version": "1.0",
      "strict": true,
      "fallback": "markdown"
    }
  },
  "sandbox": {
    "enabled": true,
    "mode": "auto",
    "image": "eino-sandbox:latest",
    "workdir": "/workspace",
    "network": "none",
    "timeout_ms": 30000,
    "env": {
      "NODE_ENV": "production"
    },
    "limits": {
        "cpu": "0.5",
        "memory": "512m",
        "network": "restricted" // 限制沙箱内的网络访问
    }
  }
}
```

## 简化请求示例

### 只包含基础字段

```json
{
  "prompt": "你是一个智能助手。",
  "models": {
    "default": {
      "provider": "openai",
      "name": "gpt-4o",
      "api_key": "sk-xxxx"
    }
  },
  "messages": [
    {"role": "user", "content": "今天天气怎么样？"}
  ]
}
```

### 包含 Skill 的请求（多模型配置）

```json
{
  "prompt": "你是一个订单客服助手。",
  "models": {
    "default": {
      "provider": "anthropic",
      "name": "claude-sonnet-4-6",
      "api_base": "https://api.anthropic.com",
      "api_key": "sk-ant-xxxx"
    },
    "rewrite": {
      "provider": "openai",
      "name": "gpt-4o-mini",
      "api_key": "sk-xxxx"
    }
  },
  "messages": [
    {"role": "user", "content": "帮我查一下订单 20240315001 的状态"}
  ],
  "skills": [
    {
      "id": "order_query",
      "name": "订单查询",
      "description": "查询订单信息",
      "instruction": "你是一个订单查询助手...",
      "scope": "chat",
      "trigger": "auto",
      "entry_script": "main.py",
      "file_path": "./skills/order_query"
    }
  ]
}
```

### 包含 MCP 的请求

```json
{
  "prompt": "你是一个数据分析助手。",
  "models": {
    "default": {
      "provider": "openai",
      "name": "gpt-4o",
      "api_key": "sk-xxxx"
    }
  },
  "messages": [
    {"role": "user", "content": "分析一下本月销售数据"}
  ],
  "mcps": [
    {
      "name": "analytics_db",
      "command": "python",
      "args": ["./mcp/analytics_server.py"],
      "env": {
        "DATABASE_URL": "postgresql://localhost:5432/analytics"
      }
    }
  ]
}
```

### 包含 RAG 知识库的请求

```json
{
  "prompt": "你是一个技术支持助手，根据提供的知识库内容回答用户问题。",
  "models": {
    "default": {
      "provider": "openai",
      "name": "gpt-4o",
      "api_key": "sk-xxxx"
    }
  },
  "messages": [
    {"role": "user", "content": "如何重置密码？"}
  ],
  "knowledge": [
    {
      "id": "kb_001",
      "name": "账号安全指南",
      "content": "重置密码步骤：1. 点击登录页的\"忘记密码\" 2. 输入注册邮箱 3. 点击发送验证码 4. 输入验证码后设置新密码",
      "score": 0.95
    }
  ]
}
```

### 流式响应请求

```json
{
  "prompt": "你是一个故事讲述者。",
  "models": {
    "default": {
      "provider": "openai",
      "name": "gpt-4o",
      "api_key": "sk-xxxx"
    }
  },
  "messages": [
    {"role": "user", "content": "给我讲一个关于小狐狸的童话故事"}
  ],
  "options": {
    "stream": true
  }
}
```

### 执行控制配置

可以通过 `options` 配置执行过程中的超时和重试策略：

```json
"options": {
  "timeout_ms": 60000,
  "retry": {
    "max_attempts": 3,
    "initial_delay_ms": 1000,
    "max_delay_ms": 10000,
    "backoff_multiplier": 2.0,
    "retryable_errors": ["timeout", "rate_limit", "server_error"]
  }
}
```

#### 超时配置

| 字段           | 类型 | 说明                     | 默认值        |
| -------------- | ---- | ------------------------ | ------------- |
| **timeout_ms** | int  | 单次请求超时时间（毫秒） | 60000 (1分钟) |

#### 重试配置

| 字段                         | 类型  | 说明                 | 默认值 |
| ---------------------------- | ----- | -------------------- | ------ |
| **retry.max_attempts**       | int   | 最大重试次数         | 3      |
| **retry.initial_delay_ms**   | int   | 初始重试延迟（毫秒） | 1000   |
| **retry.max_delay_ms**       | int   | 最大重试延迟（毫秒） | 10000  |
| **retry.backoff_multiplier** | float | 退避倍数             | 2.0    |
| **retry.retryable_errors**   | array | 可重试的错误类型     | -      |

#### 重试错误类型

| 错误类型           | 说明              |
| ------------------ | ----------------- |
| **timeout**        | 请求超时          |
| **rate_limit**     | 速率限制（429）   |
| **server_error**   | 服务器错误（5xx） |
| **network_error**  | 网络错误          |
| **empty_response** | 空响应            |

#### 执行限制配置

```json
"options": {
  "max_iterations": 10,
  "max_tool_calls": 20,
  "max_a2a_calls": 5,
  "max_total_tokens": 100000
}
```

#### 沙箱配置

可以在请求中配置沙箱执行环境，用于安全隔离的代码执行：

```json
"sandbox": {
  "enabled": true,
  "mode": "auto",
  "image": "eino-sandbox:latest",
  "workdir": "/workspace",
  "network": "none",
  "timeout_ms": 30000,
  "env": {
    "NODE_ENV": "production"
  }
}
```

| 字段           | 类型   | 说明                         | 默认值     |
| -------------- | ------ | ---------------------------- | ---------- |
| **enabled**    | bool   | 是否启用沙箱                 | true       |
| **mode**       | string | 沙箱模式: auto/docker/http   | auto       |
| **image**      | string | Docker 镜像                  | -          |
| **workdir**    | string | 工作目录                     | /workspace |
| **network**    | string | 网络模式: none/bridge/custom | none       |
| **timeout_ms** | int    | 执行超时                     | 30000      |
| **env**        | object | 环境变量                     | -          |

#### 完整执行配置示例

```json
"options": {
  "timeout_ms": 60000,
  "max_iterations": 10,
  "max_tool_calls": 20,
  "max_a2a_calls": 5,
  "sandbox": {
    "enabled": true,
    "mode": "docker",
    "image": "eino-sandbox:v1.0",
    "network": "none",
    "timeout_ms": 30000
  },
  "retry": {
    "max_attempts": 3,
    "initial_delay_ms": 1000
  }
}
```

| 字段                 | 类型 | 说明                           |
| -------------------- | ---- | ------------------------------ |
| **max_iterations**   | int  | 最大迭代次数（Agent 循环次数） |
| **max_tool_calls**   | int  | 最大工具调用次数               |
| **max_a2a_calls**    | int  | 最大 A2A Agent 调用次数        |
| **max_total_tokens** | int  | 最大 token 消耗限制            |

### 响应格式配置

可以通过 `response_format` 配置不同的响应格式：

```json
"options": {
  "response_format": {
    "type": "a2ui",
    "version": "1.0",
    "strict": true,
    "fallback": "markdown",
    "templates": {
      "weather": {
        "surfaceId": "weather_panel",
        "catalogId": "standard"
      }
    }
  }
}
```

#### 支持的响应格式

| 格式          | 说明               | 示例场景               |
| ------------- | ------------------ | ---------------------- |
| **text**      | 纯文本             | 简单问答               |
| **markdown**  | Markdown 格式      | 技术文档、代码         |
| **a2ui**      | A2UI 结构化格式    | 富文本交互、卡片、按钮 |
| **json**      | 自定义 JSON Schema | 结构化数据输出         |
| **image**     | 图片 (base64/URL)  | 图片生成               |
| **audio**     | 音频 (base64/URL)  | 语音合成               |
| **video**     | 视频 (base64/URL)  | 视频生成               |
| **multipart** | 多格式混合         | 文本+图片+音频         |

#### 响应格式示例

**文本格式 (text)**
```json
{
  "content": "您好！您的订单已发货。",
  "format": "text"
}
```

**Markdown 格式**
```json
{
  "content": "# 订单详情\n\n- 订单号：20240315001\n- 状态：**已发货**\n\n```python\ndef hello():\n    print('Hello')\n```",
  "format": "markdown"
}
```

**A2UI 格式**
```json
{
  "content": "您好！您的订单已发货。",
  "format": "a2ui",
  "action": {
    "type": "form",
    "data": {
      "buttons": [
        {"label": "查看物流", "action": "view_logistics"},
        {"label": "确认收货", "action": "confirm"}
      ]
    }
  }
}
```

**自定义 JSON Schema 格式**

如果 A2UI 不满足需求，可以定义自己的 JSON Schema：

```json
"options": {
  "response_format": {
    "type": "json",
    "schema": {
      "type": "object",
      "properties": {
        "status": {"type": "string", "enum": ["success", "error"]},
        "data": {
          "type": "object",
          "properties": {
            "order_id": {"type": "string"},
            "status": {"type": "string"},
            "amount": {"type": "number"}
          },
          "required": ["order_id", "status"]
        },
        "message": {"type": "string"}
      },
      "required": ["status", "data"]
    },
    "strict": true
  }
}
```

**图片格式**
```json
{
  "content": "这是生成的海报图片：",
  "format": "image",
  "data": {
    "type": "url",
    "url": "https://example.com/image.png"
  }
}
```

**多格式混合 (multipart)**
```json
{
  "content": "这是您的订单图表：",
  "format": "multipart",
  "parts": [
    {
      "type": "text",
      "content": "图表说明..."
    },
    {
      "type": "image",
      "data": {
        "url": "https://example.com/chart.png"
      }
    },
    {
      "type": "audio",
      "data": {
        "url": "https://example.com/voice.mp3"
      }
    }
  ]
}
```

### 多模型配置说明

| 模型 Key      | 用途           | 适用场景               |
| ------------- | -------------- | ---------------------- |
| **default**   | 默认模型       | 主对话、生成回复       |
| **rewrite**   | 问题改写       | 用户问题优化、意图识别 |
| **skill**     | Skill 执行     | 调用技能时的模型       |
| **summarize** | 总结           | 长对话摘要、内容整理   |
| **embedding** | 向量 Embedding | 知识库检索             |

**示例场景**：
- 用户问题 → rewrite 模型（gpt-4o-mini，便宜快速）改写
- 改写后 → default 模型（gpt-4o）理解意图
- 需要调用 Skill → skill 模型执行
- 结果太长 → summarize 模型（claude-haiku）总结

## A2UI 响应格式

A2UI (Agent to User Interface) 是 Google 开源的 AI 输出格式规范，用于结构化返回。

### 请求中配置 A2UI 格式

```json
"options": {
  "response_schema": {
    "type": "a2ui",
    "version": "1.0",
    "strict": true,
    "schema": {
      "type": "object",
      "properties": {
        "content": {
          "type": "string",
          "description": "回复用户的文本内容"
        },
        "action": {
          "type": "object",
          "properties": {
            "type": {
              "type": "string",
              "enum": ["none", "form", "link", "calendar", "location"]
            },
            "data": {}
          }
        },
        "card": {
          "type": "object",
          "properties": {
            "title": {"type": "string"},
            "sections": {"type": "array"},
            "actions": {"type": "array"}
          }
        }
      }
    }
  }
}
```

### A2UI 响应示例

#### 简单文本回复
```json
{
  "content": "您好！您的订单 #20240315001 已于今天上午发货，预计明天送达。",
  "action": {
    "type": "none"
  }
}
```

#### 带按钮操作
```json
{
  "content": "请问您需要查看订单详情还是物流信息？",
  "action": {
    "type": "form",
    "data": {
      "type": "button_group",
      "buttons": [
        {"label": "查看详情", "action": "view_detail", "style": "primary"},
        {"label": "查看物流", "action": "view_logistics", "style": "default"}
      ]
    }
  }
}
```

#### 卡片展示（富文本）
```json
{
  "content": "订单状态如下：",
  "card": {
    "title": "📦 订单详情",
    "sections": [
      {
        "header": "订单信息",
        "fields": [
          {"label": "订单号", "value": "20240315001"},
          {"label": "状态", "value": "已发货"},
          {"label": "金额", "value": "¥9999"}
        ]
      },
      {
        "header": "物流信息",
        "fields": [
          {"label": "快递", "value": "顺丰速运"},
          {"label": "单号", "value": "SF1234567890"}
        ]
      }
    ],
    "actions": [
      {"label": "查看物流", "type": "link", "href": "https://example.com/track"}
    ]
  }
}
```

#### 日历事件
```json
{
  "content": "已为您预约客服时间：",
  "action": {
    "type": "calendar",
    "data": {
      "title": "客服回访",
      "description": "订单 #20240315001 售后咨询",
      "startTime": "2024-03-15T14:00:00Z",
      "endTime": "2024-03-15T14:30:00Z"
    }
  }
}
```

### 响应示例

### A2UI 格式响应（推荐）

A2UI (Agent to User Interface) 是 Google 开源的 AI 输出格式规范，基于 weather agent 的实现，核心结构如下：

#### A2UI Message 类型

| 类型                 | 说明     | 示例              |
| -------------------- | -------- | ----------------- |
| **createSurface**    | 创建画布 | 创建天气卡片      |
| **deleteSurface**    | 删除画布 | 关闭面板          |
| **updateComponents** | 更新组件 | 添加/修改卡片内容 |
| **updateDataModel**  | 更新数据 | 绑定动态数据      |

#### A2UI 组件系统

| 组件       | 说明     | 属性                                   |
| ---------- | -------- | -------------------------------------- |
| **Column** | 垂直布局 | children, justify, align               |
| **Row**    | 水平布局 | children, justify, align               |
| **Card**   | 卡片容器 | child, style                           |
| **Text**   | 文本     | text (支持 path 引用), variant, weight |
| **Button** | 按钮     | label, action, style                   |

#### A2UI 配置示例

```json
"options": {
  "response_format": {
    "type": "a2ui",
    "version": "1.0",
    "strict": true,
    "templates": {
      "weather": {
        "surfaceId": "weather_panel",
        "catalogId": "standard",
        "components": [
          {"id": "root", "component": "Column", "children": ["weather_card"]},
          {"id": "weather_card", "component": "Card", "child": "weather_content"},
          {"id": "weather_content", "component": "Column", "children": ["city", "temp", "forecast"]},
          {"id": "city", "component": "Text", "text": {"path": "/cityName"}, "variant": "h3"},
          {"id": "temp", "component": "Text", "text": {"path": "/temperature"}, "variant": "h1"},
          {"id": "forecast", "component": "Card", "child": "forecast_list"}
        ]
      }
    },
    "fallback": "markdown"
  }
}
```

#### A2UI 响应结构

当请求中配置了 `response_format.type = "a2ui"` 时，返回 A2UI 格式：

```json
{
  "a2ui_messages": [
    {"createSurface": {"surfaceId": "weather_panel", "catalogId": "standard"}},
    {"updateComponents": {"surfaceId": "weather_panel", "components": [...]}},
    {"updateDataModel": {"surfaceId": "weather_panel", "path": "/", "value": {...}}}
  ],
  "metadata": {
    "model": "gpt-4o",
    "latency_ms": 1500
  }
}
```

#### 完整 A2UI 响应示例（天气）

```json
{
  "a2ui_messages": [
    {
      "createSurface": {
        "surfaceId": "weather_panel",
        "catalogId": "standard"
      }
    },
    {
      "updateComponents": {
        "surfaceId": "weather_panel",
        "components": [
          {"id": "root", "component": "Column", "children": ["weather_card"], "justify": "start"},
          {"id": "weather_card", "component": "Card", "child": "content"},
          {"id": "content", "component": "Column", "children": ["city_name", "temp", "forecast"]},
          {"id": "city_name", "component": "Text", "text": {"path": "/cityName"}, "variant": "h3"},
          {"id": "temp", "component": "Text", "text": {"path": "/temperature"}, "variant": "h1"},
          {"id": "forecast", "component": "Text", "text": {"path": "/forecast"}}
        ]
      }
    },
    {
      "updateDataModel": {
        "surfaceId": "weather_panel",
        "path": "/",
        "value": {
          "cityName": "上海",
          "temperature": "22°C",
          "forecast": "明天晴，25°C"
        }
      }
    }
  ]
}
```

### 普通文本响应

```json
{
  "content": "您好！根据订单号 20240315001 查询结果如下：\n\n📦 订单状态：**已发货**\n📅 下单时间：2024-03-15 14:30\n🎁 商品：iPhone 15 Pro Max 256G 钛金属色\n💰 订单金额：¥9999\n🚚 快递公司：顺丰速运\n📋 快递单号：SF1234567890\n\n预计送达时间：2024-03-17（明天）",
  "tool_calls": [
    {
      "tool": "get_order_detail",
      "input": {"order_id": "20240315001"},
      "output": "{\"order_id\":\"20240315001\",\"status\":\"shipped\",\"items\":[...],\"tracking_number\":\"SF1234567890\"}"
    }
  ],
  "a2a_calls": [],
  "tokens_used": 850,
  "finish_reason": "stop",
  "metadata": {
    "model": "gpt-4o",
    "latency_ms": 1500
  }
}
```

### 工具调用响应

```json
{
  "content": "",
  "tool_calls": [
    {
      "tool": "get_order_detail",
      "input": {"order_id": "20240315001"},
      "output": "{\"status\":\"shipped\",\"shipping_date\":\"2024-03-16\"}"
    }
  ],
  "a2a_calls": [],
  "tokens_used": 200,
  "finish_reason": "tool",
  "metadata": {
    "model": "gpt-4o",
    "latency_ms": 800
  }
}
```

## A2A Agent 响应格式

当请求中配置了 A2A Agent 时，响应可以包含 A2A 格式的数据：

### A2A 格式的响应

```json
{
  "content": "根据您的退款申请，已查询到订单信息...",
  "a2a_results": [
    {
      "agent_name": "payment_agent",
      "status": "completed",
      "result": {
        "artifacts": [
          {
            "type": "text",
            "content": "订单 #20240315001 已退款，退款金额 ¥9999 将于 1-3 个工作日内原路返回。",
            "mimeType": "text/plain"
          }
        ],
        "message": {
          "role": "agent",
          "parts": [
            {
              "type": "text",
              "text": "已为您处理退款申请"
            }
          ]
        }
      },
      "error": null
    }
  ],
  "metadata": {
    "model": "gpt-4o",
    "latency_ms": 2500
  }
}
```

### A2A 字段说明

| 字段                           | 类型   | 说明                       |
| ------------------------------ | ------ | -------------------------- |
| **a2a_results**                | array  | A2A Agent 调用结果列表     |
| a2a_results[].agent_name       | string | Agent 名称                 |
| a2a_results[].status           | string | 执行状态: completed/failed |
| a2a_results[].result.artifacts | array  | 返回的工件（文本/文件等）  |
| a2a_results[].result.message   | object | Agent 返回的消息           |
| a2a_results[].error            | object | 错误信息（如果有）         |

### A2A 约束配置

可以在请求中为 A2A Agent 配置约束，限制其行为：

```json
"constraints": {
  "max_calls": 5,
  "timeout_ms": 30000,
  "allowed_operations": ["query", "check_status"],
  "blocked_operations": ["refund", "cancel"],
  "max_cost_usd": 1.0,
  "allowed_resources": ["order:*"],
  "blocked_resources": ["admin:*"],
  "require_confirmation": false,
  "allowed_tools": ["http_get", "http_post"],
  "blocked_tools": ["exec", "delete"]
}
```

#### 约束字段说明

| 字段                     | 类型  | 说明             | 示例                    |
| ------------------------ | ----- | ---------------- | ----------------------- |
| **max_calls**            | int   | 最大调用次数     | 5                       |
| **timeout_ms**           | int   | 超时时间（毫秒） | 30000                   |
| **allowed_operations**   | array | 允许的操作列表   | ["query", "check"]      |
| **blocked_operations**   | array | 禁止的操作列表   | ["refund", "cancel"]    |
| **max_cost_usd**         | float | 最大花费（美元） | 1.0                     |
| **allowed_resources**    | array | 允许访问的资源   | ["order:*", "user:*"]   |
| **blocked_resources**    | array | 禁止访问的资源   | ["admin:*", "system:*"] |
| **require_confirmation** | bool  | 是否需要确认     | false                   |
| **allowed_tools**        | array | 允许使用的工具   | ["http_get"]            |
| **blocked_tools**        | array | 禁止使用的工具   | ["exec", "delete"]      |

#### 约束使用场景

**场景 1：限制支付操作**
```json
{
  "name": "payment_agent",
  "constraints": {
    "max_calls": 1,
    "allowed_operations": ["query", "check_status"],
    "blocked_operations": ["refund", "cancel", "modify"],
    "require_confirmation": true
  }
}
```

**场景 2：只读查询**
```json
{
  "name": "query_agent",
  "constraints": {
    "max_calls": 10,
    "timeout_ms": 5000,
    "allowed_operations": ["query", "search"],
    "blocked_operations": ["write", "update", "delete"],
    "allowed_resources": ["order:read*", "product:read*"]
  }
}
```

**场景 3：成本控制**
```json
{
  "name": "data_analysis_agent",
  "constraints": {
    "max_calls": 3,
    "max_cost_usd": 0.5,
    "timeout_ms": 60000
  }
}
```

### 简化 A2A 响应

如果只需要简单的文本结果：

```json
{
  "content": "已为您处理退款申请",
  "a2a_results": [
    {
      "agent_name": "payment_agent",
      "status": "completed",
      "result": "订单 #20240315001 已退款，退款金额 ¥9999"
    }
  ]
}
```
