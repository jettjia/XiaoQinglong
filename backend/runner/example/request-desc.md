# 关于执行器的参数说明

```json
{
    "endpoint": "http://localhost:18080/run",
    "model": {
        "provider": "openai",
        "name": "${OPENAI_MODEL}",
        "api_key": "${OPENAI_API_KEY}",
        "api_base": "${OPENAI_BASE_URL}"
    },
    "system_prompt": "你是一个智能助手，擅长使用工具和技能来回答用户问题。如果需要查询订单，请使用 get_order_detail 工具。如果需要上传文件到S3，可以使用 s3-upload 技能。",
    "user_message": "把当前目录下的 test.json 上传到 S3",
    "tools": [
        {
            "type": "http",
            "name": "get_order_detail",
            "description": "根据订单号获取订单详情",
            "endpoint": "http://localhost:28081/v1/orders/{order_no}",
            "method": "GET",
            "headers": {
                "Content-Type": "application/json"
            },
            "risk_level": "medium"
        },
        {
            "type": "http",
            "name": "get_product_info",
            "description": "获取产品信息",
            "endpoint": "http://localhost:8081/products/{product_id}",
            "method": "GET",
            "headers": {},
            "risk_level": "low"
        }
    ],
    "a2a": [
        {
            "name": "payment_agent",
            "endpoint": "http://localhost:28080/a2a",
            "headers": {}
        }
    ],
    "mcps": [
        {
            "name": "weather",
            "command": "go",
            "transport": "stdio",
            "args": [
                "run",
                "/home/jett/aishu/XiaoQinglong/mock/mcp"
            ],
            "env": {}
        }
    ],
    "sandbox": {
        "enabled": true,
        "mode": "docker",
        "image": "sandbox-code-interpreter:v1.0.3",
        "workdir": "/workspace",
        "timeout_ms": 120000,
        "env": {
            "PATH": "/usr/local/bin:/usr/bin:/bin"
        },
        "limits": { "cpu": "0.5", "memory": "512m" }
    },
    "options": {
        "temperature": 0.7,
        "max_tokens": 2000,
        "max_iterations": 10,
        "stream": true,
        "approval_policy": {
            "enabled": true,
            "risk_threshold": "medium", // 对于 medium 或 high 风险的操作，执行器不应直接运行，而是应该挂起任务并返回一个 pending_approval 状态给前端
            "auto_approve_tools": ["get_product_info"] // 白名单
        }
        "include_thought": true, // 是否返回模型推理过程
        "thought_format": "markdown",
        "retry": {
            "max_attempts": 3,
            "initial_delay_ms": 1000,
            "max_delay_ms": 10000,
            "backoff_multiplier": 2.0,
            "retryable_errors": [
                "timeout",
                "rate_limit",
                "server_error"
            ]
        },
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
                                "enum": [
                                    "none",
                                    "form",
                                    "link",
                                    "calendar",
                                    "location"
                                ]
                            },
                            "data": {}
                        }
                    },
                    "card": {
                        "type": "object",
                        "properties": {
                            "title": {
                                "type": "string"
                            },
                            "sections": {
                                "type": "array"
                            },
                            "actions": {
                                "type": "array"
                            }
                        }
                    }
                }
            }
        }
    },
    "skills": [
        {
            "id": "s3-upload",
            "name": "S3上传下载",
            "description": "用于上传和下载S3对象存储中的文件",
            "instruction": "你是一个S3操作助手，可以帮助用户上传文件到S3或从S3下载文件。",
            "scope": "both",
            "trigger": "manual",
            "runtime": "python3.10",
            "dependencies": ["boto3"],
            "entry_script": "python3 scripts/s3_upload.py",
            "file_path": "s3-upload"
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
            "id": "product_info",
            "name": "产品信息",
            "content": "我们的产品分为三个系列：基础版、专业版、企业版。基础版免费，专业版99元/月，企业版299元/月。",
            "score": 0.95,
             "metadata": {
                "source": "internal_wiki",
                "url": "http://wiki.company.com/products",
                "last_updated": "2024-01-01"
            }
        }
    ]
}
```