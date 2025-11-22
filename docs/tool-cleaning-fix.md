# 工具定义自动清理功能 (v3.0.1)

## 问题背景

在使用代理转发请求到 Anthropic API 时,上游服务(如 88code)发送的工具定义可能包含 Anthropic API 不支持的字段,导致 400 错误:

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "tools.2.custom.input_examples: Extra inputs are not permitted"
  }
}
```

## 错误原因

上游服务发送的工具定义包含了额外字段,例如:

```json
{
  "name": "tool_name",
  "description": "tool description",
  "input_schema": {...},
  "custom": {
    "input_examples": ["example1", "example2"]
  }
}
```

Anthropic API 只接受以下标准字段:
- `name`: 工具名称
- `description`: 工具描述
- `input_schema`: 输入 JSON Schema

其他字段(如 `custom`、`input_examples` 等)会导致请求被拒绝。

## 解决方案

在代理层添加请求体自动清理功能:

1. **位置**: `internal/proxy/proxy.go` 的 `newReverseProxy` 函数
2. **实现**: 在 `proxy.Director` 中添加中间件
3. **逻辑**:
   - 仅处理 POST/PUT 且 Content-Type 为 application/json 的请求
   - 解析请求体中的 `tools` 数组
   - 遍历每个工具定义,只保留标准字段 (`name`, `description`, `input_schema`)
   - 移除所有非标准字段
   - 重新序列化并更新请求体

## 代码实现

```go
// 清理工具定义，去除 Anthropic 未支持的字段。
cleanTools := func(body []byte) ([]byte, bool) {
    var payload map[string]any
    if err := json.Unmarshal(body, &payload); err != nil {
        return nil, false
    }
    rawTools, ok := payload["tools"].([]any)
    if !ok || len(rawTools) == 0 {
        return nil, false
    }

    changed := false
    sanitized := make([]any, 0, len(rawTools))
    for _, item := range rawTools {
        obj, ok := item.(map[string]any)
        if !ok {
            sanitized = append(sanitized, item)
            continue
        }
        cleaned := make(map[string]any, 3)
        if v, ok := obj["name"]; ok {
            cleaned["name"] = v
        }
        if v, ok := obj["description"]; ok {
            cleaned["description"] = v
        }
        if v, ok := obj["input_schema"]; ok {
            cleaned["input_schema"] = v
        }
        if len(cleaned) != len(obj) {
            changed = true
        }
        sanitized = append(sanitized, cleaned)
    }
    if !changed {
        return nil, false
    }
    payload["tools"] = sanitized
    buf, err := json.Marshal(payload)
    if err != nil {
        return nil, false
    }
    return buf, true
}
```

## 测试验证

完整测试代码: `verify/tool-cleaning/verify_tool_cleaning_pass.go`

测试用例:
1. ✓ 包含 `custom.input_examples` 字段的工具 → 成功清理
2. ✓ 标准工具定义(无额外字段) → 原样保留
3. ✓ 多个工具,部分包含额外字段 → 正确清理
4. ✓ HTTP 代理集成测试 → 代理正确工作

## 使用说明

无需额外配置,代理会自动清理所有转发请求中的非标准工具字段。

## 兼容性

- 对标准格式的请求无影响(原样转发)
- 对包含额外字段的请求自动清理
- 清理失败时原样转发,不影响正常流程
- 适用于所有通过代理转发的请求

## 更新日期

2025-11-22
