# Runner CLI

命令行工具，用于与 Runner 服务交互。

## 编译

```bash
cd backend/runner/cli
go build -o runcli .
```

## 环境变量配置

```bash
# 模型配置
export RUNNER_MODEL_DEFAULT_NAME=${OPENAI_MODEL}
export RUNNER_MODEL_DEFAULT_PROVIDER=openai
export RUNNER_MODEL_DEFAULT_APIKEY=${OPENAI_API_KEY}
export RUNNER_MODEL_DEFAULT_APIBASE=${OPENAI_BASE_URL}
export RUNNER_MODEL_DEFAULT_TEMPERATURE=0
export RUNNER_MODEL_DEFAULT_MAXTOKENS=4096
export RUNNER_MODEL_DEFAULT_TOPP=0.9

# HTTP 端点
export RUNNER_HTTP_ENDPOINT=http://localhost:18080
```

## 使用方式

### 启动 Runner 服务

```bash
cd backend/runner
go run main.go
```

### 交互式对话

```bash
./runcli chat
```

在 REPL 中支持以下命令：

| 命令                   | 说明         |
| ---------------------- | ------------ |
| `/quit`, `/exit`, `/q` | 退出         |
| `/clear`               | 清除对话历史 |
| `/history`             | 显示对话历史 |
| `/help`                | 显示帮助     |

### 单次执行

```bash
# 带参数
./runcli run "帮我写一个 Hello World"

# 从 stdin 读取
echo "Hello!" | ./runcli run
```

### 查看配置

```bash
./runcli config show
```

### 停止任务

```bash
./runcli stop <checkpoint_id>
```

## 完整示例

```bash
# 1. 编译
cd backend/runner/cli
go build -o runcli .

# 2. 配置环境变量
export RUNNER_MODEL_DEFAULT_NAME=gpt-4o
export RUNNER_MODEL_DEFAULT_APIKEY=${OPENAI_API_KEY}
export RUNNER_MODEL_DEFAULT_APIBASE=https://api.openai.com/v1

# 3. 启动 Runner 服务（另一个终端）
cd backend/runner
go run main.go

# 4. 开始对话
./runcli chat
> 你好，请帮我写一个斐波那契数列函数
> /clear
> /quit
```
