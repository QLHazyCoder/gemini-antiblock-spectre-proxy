# 构建阶段
FROM golang:1.25-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的包
RUN apk add --no-cache git ca-certificates tzdata

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o gemini-antiblock .

# 运行阶段
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata curl

# 创建非root用户
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/gemini-antiblock /app/gemini-antiblock

# 设置文件权限
RUN chown -R appuser:appgroup /app

# 切换到非root用户
USER appuser

# 设置环境变量
ENV UPSTREAM_URL_BASE=https://generativelanguage.googleapis.com
ENV ANTIBLOCK_MODEL_PREFIXES=gemini-2.5-pro
ENV PORT=8080
ENV DEBUG_MODE=false
ENV MAX_CONSECUTIVE_RETRIES=100
ENV RETRY_DELAY_MS=750
ENV SWALLOW_THOUGHTS_AFTER_RETRY=true
ENV ENABLE_RATE_LIMIT=false
ENV RATE_LIMIT_COUNT=10
ENV RATE_LIMIT_WINDOW_SECONDS=60
ENV ENABLE_PUNCTUATION_HEURISTIC=true

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:${PORT}/health || exit 1

# 运行应用
ENTRYPOINT ["/app/gemini-antiblock"]
