# Gemini Antiblock Proxy (Go 版本)

![Gemini Antiblock Spectre Proxy Overview](docs/images/proxy-overview.png)

这是一个用 Go 语言重写的 Gemini API 代理服务器，具有强大的流式重试和标准化错误响应功能。它可以处理模型的"思考"过程，并在重试后过滤思考内容以保持干净的输出流。

## 功能特性

- **流式响应处理**: 支持 Server-Sent Events (SSE)流式响应
- **智能重试机制**: 当流被中断时自动重试，最多支持 100 次连续重试
- **思考内容过滤**: 可以在重试后过滤模型的思考过程，保持输出的整洁
- **标准化错误响应**: 提供符合 Google API 标准的错误响应格式
- **CORS 支持**: 完整的跨域资源共享支持
- **速率限制**: 可配置的请求速率限制功能
- **详细日志记录**: 支持调试模式和详细的操作日志

## 快速开始

### 使用 Docker（推荐）

#### 方式一：使用预构建镜像

```bash
# 拉取并运行
docker run -d \
  --name gemini-antiblock \
  -p 8080:8080 \
  -e UPSTREAM_URL_BASE=https://generativelanguage.googleapis.com \
  ghcr.io/qlhazycoder/gemini-antiblock-spectre-proxy:latest
```

#### 方式二：使用 Docker Compose

```bash
git clone https://github.com/QLHazyCoder/gemini-antiblock-spectre-proxy.git
cd gemini-antiblock-spectre-proxy
docker-compose up -d
```

> 首次运行前先执行 `cp .env.example .env` 并填写所需变量（SpectreProxy / API Key 等），Compose 会自动读取。

#### 方式三：本地构建

```bash
git clone https://github.com/QLHazyCoder/gemini-antiblock-spectre-proxy.git
cd gemini-antiblock-spectre-proxy
docker build -t gemini-antiblock-spectre-proxy .
docker run -d --name gemini-antiblock -p 8080:8080 gemini-antiblock-spectre-proxy
```

### 从源码运行

```bash
# 前置要求：Go 1.21+
git clone https://github.com/QLHazyCoder/gemini-antiblock-spectre-proxy.git
cd gemini-antiblock-spectre-proxy
go mod download
go run main.go
```

## 配置

### 环境变量

| 变量名                         | 默认值                                      | 描述                       |
| ------------------------------ | ------------------------------------------- | -------------------------- |
| `UPSTREAM_URL_BASE`            | `https://generativelanguage.googleapis.com` | Gemini API 的基础 URL；为空且配置 Spectre 时将自动拼接 |
| `SPECTRE_PROXY_WORKER_URL`     | *(空)*                                      | SpectreProxy Worker 地址（可选，可填写多个，自动轮询）    |
| `SPECTRE_PROXY_AUTH_TOKEN`     | *(空)*                                      | SpectreProxy 认证 Token（可选）    |
| `ANTIBLOCK_MODEL_PREFIXES`     | `gemini-2.5-pro`                            | 需启用抗断流的模型前缀（逗号分隔） |
| `PORT`                         | `8080`                                      | 服务器监听端口             |
| `DEBUG_MODE`                   | `true`                                      | 是否启用调试日志           |
| `MAX_CONSECUTIVE_RETRIES`      | `100`                                       | 流中断时的最大连续重试次数 |
| `RETRY_DELAY_MS`               | `750`                                       | 重试间隔时间（毫秒）       |
| `SWALLOW_THOUGHTS_AFTER_RETRY` | `true`                                      | 重试后是否过滤思考内容     |
| `ENABLE_RATE_LIMIT`            | `false`                                     | 是否启用速率限制           |
| `RATE_LIMIT_COUNT`             | `10`                                        | 速率限制请求数             |
| `RATE_LIMIT_WINDOW_SECONDS`    | `60`                                        | 速率限制窗口时间（秒）     |
| `ENABLE_PUNCTUATION_HEURISTIC` | `true`                                      | 启用句末标点启发式优化     |

> 💡 如果通过 Cloudflare SpectreProxy 中转，可在 `.env` 中额外声明 `SPECTRE_PROXY_WORKER_URL` 与 `SPECTRE_PROXY_AUTH_TOKEN`，并将 `UPSTREAM_URL_BASE` 留空，应用会自动拼接 `https://<WORKER>/<AUTH_TOKEN>/gemini`。`SPECTRE_PROXY_WORKER_URL` 支持逗号、分号或换行分隔多个地址，系统会自动进行轮询转发，以分散 Cloudflare 免费额度的压力。

### 配置文件

从示例文件创建配置：

```bash
cp .env.example .env
```

### Docker 完整配置示例

```bash
docker run -d \
  --name gemini-antiblock \
  -p 8080:8080 \
  -e UPSTREAM_URL_BASE=https://generativelanguage.googleapis.com \
  -e ANTIBLOCK_MODEL_PREFIXES=gemini-2.5-pro \
  -e PORT=8080 \
  -e DEBUG_MODE=false \
  -e MAX_CONSECUTIVE_RETRIES=100 \
  -e RETRY_DELAY_MS=750 \
  -e SWALLOW_THOUGHTS_AFTER_RETRY=true \
  -e ENABLE_RATE_LIMIT=false \
  -e RATE_LIMIT_COUNT=10 \
  -e RATE_LIMIT_WINDOW_SECONDS=60 \
  -e ENABLE_PUNCTUATION_HEURISTIC=true \
  ghcr.io/qlhazycoder/gemini-antiblock-spectre-proxy:latest
```

## 使用方法

代理服务器启动后，你可以将 Gemini API 的请求发送到这个代理服务器。代理会自动：

1. 转发请求到上游 Gemini API
2. 处理流式响应
3. 在流中断时自动重试
4. 注入系统提示确保响应以`[done]`结尾
5. 过滤重试后的思考内容（如果启用）

### 示例请求

```bash
curl "http://127.0.0.1:8080/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse" \
   -H "x-goog-api-key: $GEMINI_API_KEY" \
   -H 'Content-Type: application/json' \
   -X POST --no-buffer  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [
          {
            "text": "Hello"
          }
        ]
      }
    ],
    "generationConfig": {
      "thinkingConfig": {
        "includeThoughts": true
      }
    }
  }'
```

### 健康检查

```bash
curl http://localhost:8080/health
```

## 项目结构

```
gemini-antiblock-spectre-proxy/
├── main.go                 # 主程序入口
├── config/
│   └── config.go          # 配置管理
├── logger/
│   └── logger.go          # 日志记录
├── handlers/
│   ├── errors.go          # 错误处理和CORS
│   ├── health.go          # 健康检查
│   ├── proxy.go           # 代理处理逻辑
│   └── ratelimiter.go     # 速率限制
├── streaming/
│   ├── sse.go             # SSE流处理
│   └── retry.go           # 重试逻辑
├── mock-server/           # 测试模拟服务器
├── Dockerfile             # Docker构建文件
├── docker-compose.yml     # Docker Compose配置
└── README.md              # 项目文档
```

## 高级功能

### SpectreProxy Worker（可选）

仓库附带 `SpectreProxy/` 目录，提供基于 Cloudflare Workers 原生 Socket 接入的 SpectreProxy 实现，可构建 `https://<WORKER>/<AUTH_TOKEN>/gemini` 的中转上游。

使用方式：

1. 按 `SpectreProxy/README.md` 的说明部署 `AIGatewayWithSocks.js`（可以一键部署多个 Worker）。
2. 在 `.env` 中填写 `SPECTRE_PROXY_WORKER_URL`（支持多个地址）与 `SPECTRE_PROXY_AUTH_TOKEN`，并将 `UPSTREAM_URL_BASE` 留空；应用会自动拼接并轮询 SpectreProxy 上游。
3. 若不使用 SpectreProxy，可直接设置 `UPSTREAM_URL_BASE` 为官方 `https://generativelanguage.googleapis.com` 或任意自定义上游。

> SpectreProxy 使用 MIT 许可证并保留原作者 Davidasx 的版权，请阅读目录内 README 以了解更多功能与限制。

### 重试机制

当检测到以下情况时，代理会自动重试：

1. **流中断**: 流意外结束而没有完成标记
2. **内容被阻止**: 检测到内容被过滤或阻止
3. **思考中完成**: 在思考块中检测到完成标记（无效状态）
4. **异常完成原因**: 非正常的完成原因
5. **不完整响应**: 响应看起来不完整

重试时会：

- 保留已生成的文本作为上下文
- 构建继续对话的新请求
- 在达到最大重试次数后返回错误

### 日志记录

代理提供三个级别的日志：

- **DEBUG**: 详细的调试信息（仅在调试模式下显示）
- **INFO**: 一般信息和操作状态
- **ERROR**: 错误信息和异常

### 测试和开发

项目包含一个 Mock Server 用于测试，支持多种测试场景：

```bash
cd mock-server
go run main.go
```

详细测试说明请参考 [`mock-server/README.md`](mock-server/README.md)。

## 生产部署

### 生产环境建议

1. **使用特定版本标签**

   ```bash
   docker pull ghcr.io/qlhazycoder/gemini-antiblock-spectre-proxy:v1.0.0
   ```

2. **设置资源限制**

   ```bash
   docker run -d \
     --name gemini-antiblock \
     --memory=256m \
     --cpus=0.5 \
     -p 8080:8080 \
     ghcr.io/qlhazycoder/gemini-antiblock-spectre-proxy:v1.0.0
   ```

3. **启用速率限制**

   ```bash
   -e ENABLE_RATE_LIMIT=true \
   -e RATE_LIMIT_COUNT=100 \
   -e RATE_LIMIT_WINDOW_SECONDS=60
   ```

4. **配置监控**
   - 健康检查：`/health` 端点
   - 日志轮转：避免日志文件过大
   - 重启策略：确保服务高可用

### 多架构支持

Docker 镜像支持：

- `linux/amd64` (x86_64)
- `linux/arm64` (ARM64)

### CI/CD

项目使用 GitHub Actions 自动构建和发布：

- **触发条件**：推送到 `main`/`master` 分支或创建标签
- **构建平台**：支持 `linux/amd64` 和 `linux/arm64`
- **发布位置**：`ghcr.io/qlhazycoder/gemini-antiblock-spectre-proxy`

## 许可证

项目在 [MIT License](LICENSE) 下发布，保留原始作者 Davidasx 的版权声明，并追加本仓库的修改版权信息。

## 原始版本

这是基于 Cloudflare Worker 版本的 Go 语言重写版本。原始 JavaScript 版本提供了相同的功能，但运行在 Cloudflare Workers 平台上。
