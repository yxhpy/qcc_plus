#!/bin/bash
# 项目维护脚本

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"

echo "🔧 运行项目维护..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 1. 更新模块注册表
echo "1️⃣  更新模块注册表"
"$PROJECT_ROOT/.claude/scripts/update-registry.sh"
echo ""

# 2. 检查测试覆盖率
echo "2️⃣  检查测试覆盖率"
cd "$PROJECT_ROOT"
go test -cover ./... 2>&1 | grep "coverage:" || echo "  无测试覆盖率数据"
echo ""

# 3. 检查代码格式
echo "3️⃣  检查代码格式"
if command -v gofmt &> /dev/null; then
    unformatted=$(gofmt -l . 2>/dev/null | grep -v vendor || true)
    if [ -z "$unformatted" ]; then
        echo "  ✅ 代码格式正确"
    else
        echo "  ⚠️  以下文件需要格式化:"
        echo "$unformatted"
    fi
else
    echo "  ⚠️  gofmt 未安装"
fi
echo ""

# 4. 检查 Git 状态
echo "4️⃣  检查 Git 状态"
if git status --short | grep -q .; then
    echo "  ⚠️  有未提交的变更:"
    git status --short
else
    echo "  ✅ 工作区干净"
fi
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ 维护完成"
