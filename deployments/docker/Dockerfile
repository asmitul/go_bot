# 构建阶段
FROM golang:1.25-alpine AS builder

# 安装必要工具
RUN apk add --no-cache git make

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/bin/bot \
    ./cmd/bot

# 运行阶段
FROM alpine:latest

# 安装 ca-certificates（HTTPS 需要）
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=UTC

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/bin/bot .

# 创建非 root 用户
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pidof bot || exit 1

# 运行应用
CMD ["./bot"]