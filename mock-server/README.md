# Mock Server for Testing

这是一个用于测试 Gemini Antiblock 代理的模拟服务器。它提供了多种测试场景来验证代理服务器在不同情况下的行为。

## 功能特性

- **多种测试用例**: 支持 3 种不同的测试场景
- **随机延迟**: 模拟真实 API 的响应延迟（50-200ms 之间）
- **流式响应**: 支持 Server-Sent Events (SSE)格式的流式响应
- **思考内容**: 模拟包含思考过程的响应
- **CORS 支持**: 完整的跨域资源共享支持

## 测试用例

### 测试用例 1: 无结束标记 (`/type-1`)

- **描述**: 输出思考部分，但不在响应结尾返回 `[done]` 标记
- **用途**: 测试代理服务器处理不完整流的能力
- **路径**: `/type-1/...`

### 测试用例 2: 分割结束标记 (`/type-2`)

- **描述**: 将 `[done]` 标记分割成多个块发送（如 `[do` 和 `ne]`）
- **用途**: 测试代理服务器处理分割标记的能力
- **路径**: `/type-2/...`

### 测试用例 3: 空响应 (`/type-3`)

- **描述**: 返回空响应
- **用途**: 测试代理服务器处理空响应的能力
- **路径**: `/type-3/...`

## 安装和运行

### 前置要求

- Go 1.21 或更高版本

### 安装依赖

```bash
cd mock-server
go mod tidy
```

### 运行服务器

```bash
go run main.go
```

服务器将在端口 8081 上启动。

## 使用方法

### 健康检查

```bash
curl http://localhost:8081/health
```

### 流式请求示例

```bash
# 测试用例 1: 无结束标记
curl -X POST "http://localhost:8081/type-1/v1beta/models/gemini-pro:streamGenerateContent" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"parts": [{"text": "Hello"}]}]}'

# 测试用例 2: 分割结束标记
curl -X POST "http://localhost:8081/type-2/v1beta/models/gemini-pro:streamGenerateContent" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"parts": [{"text": "Hello"}]}]}'

# 测试用例 3: 空响应
curl -X POST "http://localhost:8081/type-3/v1beta/models/gemini-pro:streamGenerateContent" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"parts": [{"text": "Hello"}]}]}'
```

### 非流式请求示例

```bash
# 普通POST请求
curl -X POST "http://localhost:8081/type-1/v1beta/models/gemini-pro:generateContent" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"parts": [{"text": "Hello"}]}]}'
```

## 与主代理服务器一起测试

1. 启动模拟服务器（端口 8081）:

```bash
cd mock-server
go run main.go
```

2. 启动主代理服务器，将其配置为指向模拟服务器的不同测试用例:

```bash
cd ..

# 配置指向测试用例1
UPSTREAM_URL_BASE=http://localhost:8081/type-1 go run main.go

# 或者配置指向测试用例2
UPSTREAM_URL_BASE=http://localhost:8081/type-2 go run main.go

# 或者配置指向测试用例3
UPSTREAM_URL_BASE=http://localhost:8081/type-3 go run main.go
```

3. 通过主代理服务器发送请求:

```bash
curl -X POST "http://localhost:8080/v1beta/models/gemini-pro:streamGenerateContent" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"parts": [{"text": "Test message"}]}]}'
```

现在您可以通过设置不同的 `UPSTREAM_URL_BASE` 来测试不同的场景，而不需要在请求中添加查询参数。

## 响应格式

### 流式响应格式 (SSE)

```
data: {"candidates":[{"content":{"parts":[{"text":"Response chunk","thought":false}]}}]}

data: {"candidates":[{"content":{"parts":[{"text":"Thinking...","thought":true}]}}]}
```

### 非流式响应格式 (JSON)

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "Complete response text"
          }
        ]
      },
      "finishReason": "STOP"
    }
  ]
}
```

## 日志输出

模拟服务器会输出详细的日志信息，包括：

- 接收到的请求类型和测试用例
- 每个测试用例的执行过程
- 响应块的发送情况

这些日志有助于调试和验证代理服务器的行为。
