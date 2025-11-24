FROM golang:1.21 AS build
WORKDIR /app
COPY . .
RUN go mod download
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X 'qcc_plus/internal/version.Version=${VERSION}' -X 'qcc_plus/internal/version.GitCommit=${GIT_COMMIT}' -X 'qcc_plus/internal/version.BuildDate=${BUILD_DATE}'" \
    -o /app/ccproxy ./cmd/cccli

# 下载 cloudflared 和 Docker CLI
FROM debian:bookworm-slim AS tools
RUN apt-get update && apt-get install -y curl ca-certificates && \
    # 下载 cloudflared
    curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o /cloudflared && \
    chmod +x /cloudflared && \
    # 下载 Docker CLI (仅客户端，不包含 daemon)
    curl -fsSL https://download.docker.com/linux/static/stable/x86_64/docker-24.0.7.tgz -o docker.tgz && \
    tar xzvf docker.tgz --strip 1 -C /usr/local/bin docker/docker && \
    chmod +x /usr/local/bin/docker && \
    rm docker.tgz

# 使用 debian-slim 作为运行时基础镜像（而不是 distroless），以支持 shell 和 entrypoint 脚本
FROM debian:bookworm-slim
WORKDIR /app

# 安装运行时依赖
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# 复制二进制文件
COPY --from=build /app/ccproxy /usr/local/bin/ccproxy
COPY --from=tools /cloudflared /usr/local/bin/cloudflared
COPY --from=tools /usr/local/bin/docker /usr/local/bin/docker

# 复制 Claude CLI 验证镜像的 Dockerfile 和脚本
COPY verify/claude_code_cli/Dockerfile.verify_pass /app/claude-cli/Dockerfile
COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

EXPOSE 8000
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["proxy"]
