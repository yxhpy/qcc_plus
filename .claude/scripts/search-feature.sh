#!/bin/bash
# 搜索现有功能，防止重复造轮子

set -e

if [ -z "$1" ]; then
    echo "用法: $0 <关键词>"
    echo "示例: $0 健康检查"
    exit 1
fi

KEYWORD="$1"
PROJECT_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"

echo "🔍 搜索功能: $KEYWORD"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 搜索模块注册表
echo "📦 模块注册表匹配:"
if [ -f "$PROJECT_ROOT/docs/modules/REGISTRY.md" ]; then
    rg -i "$KEYWORD" "$PROJECT_ROOT/docs/modules/REGISTRY.md" --color=always || echo "  无匹配"
else
    echo "  模块注册表不存在，请先运行 update-registry.sh"
fi
echo ""

# 搜索 API 索引
echo "🔧 API 索引匹配:"
if [ -f "$PROJECT_ROOT/docs/api/INDEX.md" ]; then
    rg -i "$KEYWORD" "$PROJECT_ROOT/docs/api/INDEX.md" --color=always || echo "  无匹配"
else
    echo "  API 索引不存在"
fi
echo ""

# 搜索源代码
echo "💻 源代码匹配:"
rg -i "$KEYWORD" "$PROJECT_ROOT/internal/" \
    --type go \
    --max-count 5 \
    --color=always \
    --heading \
    --line-number || echo "  无匹配"
echo ""

# 搜索文档
echo "📚 文档匹配:"
rg -i "$KEYWORD" "$PROJECT_ROOT/docs/" \
    --type md \
    --max-count 3 \
    --color=always \
    --heading \
    --line-number || echo "  无匹配"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "💡 提示: 如果找到现有功能，请复用而非重新实现"
