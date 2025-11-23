#!/bin/bash

set -e

# Docker Hub 发布脚本
# 用法: ./scripts/publish-docker.sh <username> <version>
# 示例: ./scripts/publish-docker.sh myusername v1.0.0

# 检查参数
if [ $# -ne 2 ]; then
    echo "用法: $0 <dockerhub-username> <version>"
    echo "示例: $0 myusername v1.0.0"
    exit 1
fi

DOCKER_USERNAME="$1"
VERSION="$2"
IMAGE_NAME="qcc_plus"
FULL_IMAGE_NAME="${DOCKER_USERNAME}/${IMAGE_NAME}"

echo "=========================================="
echo "Docker Hub 发布准备"
echo "=========================================="
echo "镜像名称: ${FULL_IMAGE_NAME}"
echo "版本标签: ${VERSION}"
echo "=========================================="

# 检查是否已登录 Docker Hub
echo ""
echo "检查 Docker Hub 登录状态..."
if ! docker info | grep -q "Username: ${DOCKER_USERNAME}"; then
    echo "请先登录 Docker Hub:"
    docker login
fi

# 构建 Docker 镜像
echo ""
echo "步骤 1: 构建 Docker 镜像..."
docker build -t "${FULL_IMAGE_NAME}:${VERSION}" .

# 同时打上 latest 标签
echo ""
echo "步骤 2: 添加 latest 标签..."
docker tag "${FULL_IMAGE_NAME}:${VERSION}" "${FULL_IMAGE_NAME}:latest"

# 推送到 Docker Hub
echo ""
echo "步骤 3: 推送镜像到 Docker Hub..."
docker push "${FULL_IMAGE_NAME}:${VERSION}"
docker push "${FULL_IMAGE_NAME}:latest"

echo ""
echo "=========================================="
echo "发布完成！"
echo "=========================================="
echo "镜像已发布到:"
echo "  - ${FULL_IMAGE_NAME}:${VERSION}"
echo "  - ${FULL_IMAGE_NAME}:latest"
echo ""
echo "用户可以通过以下命令拉取:"
echo "  docker pull ${FULL_IMAGE_NAME}:${VERSION}"
echo "  docker pull ${FULL_IMAGE_NAME}:latest"
echo "=========================================="
