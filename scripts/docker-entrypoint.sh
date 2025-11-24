#!/bin/bash
set -e

echo "=== qcc_plus Docker Entrypoint ==="

# 检查 Docker 是否可用（通过挂载的 socket）
if [ -S /var/run/docker.sock ]; then
    echo "✓ Docker socket detected at /var/run/docker.sock"

    # 检查 Docker CLI 是否可用
    if command -v docker &> /dev/null; then
        echo "✓ Docker CLI available"

        # 测试 Docker 连接
        if docker version &> /dev/null; then
            echo "✓ Docker daemon accessible"

            # 检查 claude-code-cli-verify 镜像是否存在
            if ! docker images | grep -q claude-code-cli-verify; then
                echo "⚠ Claude CLI verify image not found, building..."

                # 构建镜像
                if [ -f /app/claude-cli/Dockerfile ]; then
                    cd /app/claude-cli
                    docker build -f Dockerfile -t claude-code-cli-verify . 2>&1 | head -20

                    if [ $? -eq 0 ]; then
                        echo "✓ Claude CLI verify image built successfully"
                    else
                        echo "✗ Failed to build Claude CLI verify image"
                        echo "  CLI health check will not be available"
                    fi
                else
                    echo "✗ Dockerfile not found at /app/claude-cli/Dockerfile"
                fi
            else
                echo "✓ Claude CLI verify image already exists"
            fi
        else
            echo "✗ Cannot connect to Docker daemon"
            echo "  Make sure Docker socket is properly mounted"
            echo "  CLI health check will not be available"
        fi
    else
        echo "✗ Docker CLI not found in container"
    fi
else
    echo "⚠ Docker socket not mounted at /var/run/docker.sock"
    echo "  CLI health check will not be available"
    echo "  To enable: mount -v /var/run/docker.sock:/var/run/docker.sock"
fi

echo "=== Starting ccproxy ==="
echo

# 启动主程序，传递所有参数
exec /usr/local/bin/ccproxy "$@"
