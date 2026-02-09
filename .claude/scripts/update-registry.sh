#!/bin/bash
# 更新模块注册表

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
REGISTRY_FILE="$PROJECT_ROOT/docs/modules/REGISTRY.md"

echo "📝 更新模块注册表..."
echo ""

# 备份现有注册表
if [ -f "$REGISTRY_FILE" ]; then
    cp "$REGISTRY_FILE" "$REGISTRY_FILE.backup"
    echo "✅ 已备份现有注册表"
fi

# 扫描 internal/ 目录
echo "🔍 扫描 internal/ 目录..."
echo ""

# 生成注册表头部
cat > "$REGISTRY_FILE" << 'EOF'
# 模块注册表

> 自动生成 | 最后更新: $(date '+%Y-%m-%d %H:%M:%S')
> 运行 `./.claude/scripts/update-registry.sh` 更新

## 说明

本注册表记录项目中所有模块的信息，防止重复造轮子。

**使用方法**:
1. 开发前先搜索: `./.claude/scripts/search-feature.sh "功能关键词"`
2. 查看本注册表了解现有模块
3. 复用现有功能而非重新实现

## 核心模块

EOF

# 扫描每个模块目录
for dir in "$PROJECT_ROOT"/internal/*/; do
    if [ -d "$dir" ]; then
        module_name=$(basename "$dir")
        echo "  扫描: $module_name"

        # 添加模块信息到注册表
        cat >> "$REGISTRY_FILE" << EOF

### $module_name
- **路径**: \`internal/$module_name/\`
- **功能**: （待补充）
- **主要文件**:
EOF

        # 列出主要 Go 文件
        for file in "$dir"*.go; do
            if [ -f "$file" ] && [[ ! "$file" =~ _test\.go$ ]]; then
                filename=$(basename "$file")
                echo "  - \`$filename\`" >> "$REGISTRY_FILE"
            fi
        done

        # 检查是否有测试
        if ls "$dir"*_test.go 1> /dev/null 2>&1; then
            echo "- **测试**: ✅ 有测试" >> "$REGISTRY_FILE"
        else
            echo "- **测试**: ❌ 无测试" >> "$REGISTRY_FILE"
        fi
    fi
done

echo ""
echo "✅ 模块注册表已更新: $REGISTRY_FILE"
echo ""
echo "💡 提示: 请手动补充各模块的功能描述和主要 API"
