FROM golang:1.24 AS build
WORKDIR /app
COPY . .
RUN go mod download
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X 'qcc_plus/internal/version.Version=${VERSION}' -X 'qcc_plus/internal/version.GitCommit=${GIT_COMMIT}' -X 'qcc_plus/internal/version.BuildDate=${BUILD_DATE}'" \
    -o /app/ccproxy ./cmd/cccli

# 下载 cloudflared
FROM debian:bookworm-slim AS tools
RUN apt-get update && apt-get install -y curl ca-certificates && \
    curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o /cloudflared && \
    chmod +x /cloudflared

# 使用 debian-slim 作为运行时基础镜像（而不是 distroless），以支持 shell 和 entrypoint 脚本
FROM debian:bookworm-slim
WORKDIR /app

# 安装运行时依赖（包括 Node.js 20，用于 Claude CLI）
# 注意：NodeSource 的 nodejs 包已包含 npm，不需要单独安装
RUN apt-get update && apt-get install -y ca-certificates curl gnupg && \
    mkdir -p /etc/apt/keyrings && \
    curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg && \
    echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_20.x nodistro main" > /etc/apt/sources.list.d/nodesource.list && \
    apt-get update && apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# 安装 Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code@latest && claude --version

# 复制二进制文件和资源
COPY --from=build /app/ccproxy /usr/local/bin/ccproxy
COPY --from=tools /cloudflared /usr/local/bin/cloudflared
COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh
COPY CHANGELOG.md /app/CHANGELOG.md
RUN chmod +x /app/docker-entrypoint.sh

EXPOSE 8000
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["proxy"]
