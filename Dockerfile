# ============================================
# DocHub 文库系统 Dockerfile (多阶段构建)
# ============================================

# ---- Stage 1: 编译阶段 ----
FROM golang:1.23-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git gcc musl-dev

WORKDIR /build

# 先复制 go.mod 和 go.sum，利用 Docker 层缓存
COPY go.mod go.sum ./

# 设置 Go 代理（国内环境加速）
ENV GOPROXY=https://goproxy.cn,direct
ENV GO111MODULE=on
ENV CGO_ENABLED=0

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译二进制文件
RUN go build -o DocHub .

# ============================================
# ---- Stage 2: 运行阶段 ----
FROM alpine:3.20

LABEL maintainer="DocHub"
LABEL description="DocHub 文库系统 - 含支付宝/微信支付"

# 安装必要的运行时依赖
RUN apk add --no-cache ca-certificates tzdata netcat-openbsd && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# 从编译阶段复制二进制文件
COPY --from=builder /build/DocHub .

# 复制运行时需要的目录
COPY conf/ ./conf/
COPY views/ ./views/
COPY static/ ./static/
COPY dictionary/ ./dictionary/

# 创建可写目录
RUN mkdir -p cache/session logs upload runtime

# 暴露端口
EXPOSE 8090

# 入口脚本
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENTRYPOINT ["/docker-entrypoint.sh"]
