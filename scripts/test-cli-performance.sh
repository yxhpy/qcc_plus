#!/bin/bash
# CLI 健康检查性能测试脚本
# 测试不同参数组合的响应速度

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置（请根据实际情况修改）
API_KEY="${ANTHROPIC_API_KEY:-sk-ant-your-key}"
BASE_URL="${ANTHROPIC_BASE_URL:-https://api.anthropic.com}"
MODEL="${TEST_MODEL:-claude-haiku-4-5-20251001}"
RUNS=3  # 每个配置运行的次数

echo "========================================"
echo "CLI 健康检查性能测试"
echo "========================================"
echo "Model: $MODEL"
echo "Runs per config: $RUNS"
echo ""

# 导出环境变量
export ANTHROPIC_API_KEY="$API_KEY"
export ANTHROPIC_BASE_URL="$BASE_URL"
export ANTHROPIC_AUTH_TOKEN="$API_KEY"

# 测试函数
test_command() {
    local name="$1"
    shift
    local args=("$@")

    echo -e "${YELLOW}测试配置: $name${NC}"
    echo "命令: claude ${args[*]}"

    local total_time=0
    local success_count=0

    for i in $(seq 1 $RUNS); do
        echo -n "  Run $i/$RUNS: "

        # 记录开始时间
        local start=$(date +%s%3N)

        # 执行命令
        if output=$(timeout 30s claude "${args[@]}" 2>&1); then
            local end=$(date +%s%3N)
            local duration=$((end - start))
            total_time=$((total_time + duration))
            success_count=$((success_count + 1))
            echo -e "${GREEN}成功${NC} - ${duration}ms"
            echo "    输出: $(echo "$output" | tr -d '\n' | cut -c1-50)..."
        else
            echo -e "${RED}失败${NC}"
        fi

        # 避免频繁请求
        sleep 1
    done

    if [ $success_count -gt 0 ]; then
        local avg_time=$((total_time / success_count))
        echo -e "  ${GREEN}平均响应时间: ${avg_time}ms${NC}"
        echo -e "  成功率: $success_count/$RUNS"
    else
        echo -e "  ${RED}所有测试都失败${NC}"
    fi

    echo ""
}

# ================================
# 测试场景
# ================================

echo "========================================"
echo "1. 当前实现（基线）"
echo "========================================"
test_command "当前实现" \
    -p "say ok" \
    --tools "" \
    --model "$MODEL"

echo "========================================"
echo "2. 添加 --max-turns 1"
echo "========================================"
test_command "限制最大轮数" \
    -p "say ok" \
    --tools "" \
    --model "$MODEL" \
    --max-turns 1

echo "========================================"
echo "3. 添加 --dangerously-skip-permissions"
echo "========================================"
test_command "跳过权限检查" \
    -p "say ok" \
    --tools "" \
    --model "$MODEL" \
    --dangerously-skip-permissions

echo "========================================"
echo "4. 组合优化 (max-turns + skip-permissions)"
echo "========================================"
test_command "组合优化" \
    -p "say ok" \
    --tools "" \
    --model "$MODEL" \
    --max-turns 1 \
    --dangerously-skip-permissions

echo "========================================"
echo "5. 更短的 prompt: 'ok'"
echo "========================================"
test_command "更短prompt-ok" \
    -p "ok" \
    --tools "" \
    --model "$MODEL" \
    --max-turns 1 \
    --dangerously-skip-permissions

echo "========================================"
echo "6. 最短 prompt: '1'"
echo "========================================"
test_command "最短prompt-1" \
    -p "1" \
    --tools "" \
    --model "$MODEL" \
    --max-turns 1 \
    --dangerously-skip-permissions

echo "========================================"
echo "7. 指定输出格式: text"
echo "========================================"
test_command "指定输出格式" \
    -p "ok" \
    --tools "" \
    --model "$MODEL" \
    --max-turns 1 \
    --dangerously-skip-permissions \
    --output-format text

echo "========================================"
echo "8. 使用别名模型: haiku"
echo "========================================"
test_command "模型别名" \
    -p "ok" \
    --tools "" \
    --model haiku \
    --max-turns 1 \
    --dangerously-skip-permissions

echo "========================================"
echo "9. 完全优化组合"
echo "========================================"
test_command "完全优化" \
    -p "1" \
    --tools "" \
    --model haiku \
    --max-turns 1 \
    --dangerously-skip-permissions \
    --output-format text

echo "========================================"
echo "测试完成！"
echo "========================================"
echo ""
echo "建议："
echo "1. 查看上面的平均响应时间，选择最快的配置"
echo "2. 确认输出内容是否符合预期（非空即可）"
echo "3. 在生产环境测试选定的配置"
echo ""
