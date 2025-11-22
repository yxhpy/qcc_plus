# 节点禁用/启用功能

## 功能概述

新增了手动禁用/启用节点的功能，与现有的自动失败检测（Failed）互补：

- **Failed（自动）**：系统检测到节点连续失败后自动标记，探活成功后自动恢复
- **Disabled（手动）**：用户手动禁用节点，需要手动启用才能恢复
- **两种状态都会阻止节点被自动选择使用**

## 使用场景

1. **临时维护**：节点需要维护时，手动禁用避免流量转发
2. **成本控制**：暂时不想使用某个高成本节点
3. **测试环境**：临时禁用生产节点，只使用测试节点
4. **逐步迁移**：逐个启用新节点，验证后再禁用旧节点

## 功能说明

### 管理页面

#### 1. 状态显示
在节点列表中，状态列会显示：
- **Active**（绿色）- 当前活跃节点
- **Failed**（红色）- 系统自动标记失败
- **已禁用**（橙色/灰色）- 用户手动禁用

#### 2. 操作按钮
每个节点有以下操作：
- **禁用** - 手动禁用节点（橙色按钮）
- **启用** - 手动启用已禁用的节点（绿色按钮）
- **切换** - 切换为活跃节点（禁用的节点无此按钮）
- **编辑** - 编辑节点配置
- **删除** - 删除节点

#### 3. 过滤功能
状态过滤器新增选项：
- 全部状态
- 仅活跃
- 仅失败
- 健康率 ≥ 90%
- **已禁用**（新增）

### API 端点

#### 禁用节点
```bash
POST /admin/api/nodes/disable
Content-Type: application/json

{
  "id": "node-id"
}
```

响应：
```json
{
  "disabled": "node-id"
}
```

#### 启用节点
```bash
POST /admin/api/nodes/enable
Content-Type: application/json

{
  "id": "node-id"
}
```

响应：
```json
{
  "enabled": "node-id"
}
```

## 数据库变更

### 新增字段
在 `nodes` 表中添加：
```sql
disabled BOOLEAN DEFAULT FALSE
```

### 迁移说明
- 服务启动时会自动添加 `disabled` 列
- 已有节点的 `disabled` 默认为 `false`
- 不需要手动迁移数据

## 行为说明

### 节点选择逻辑
在选择最佳节点时，系统会跳过：
1. `Failed = true` 的节点（自动失败）
2. `Disabled = true` 的节点（手动禁用）

```go
for id, n := range p.nodes {
    if n.Failed || n.Disabled {
        continue  // 跳过失败或禁用的节点
    }
    // 选择权重最高的健康节点
}
```

### 状态优先级
- **Disabled 优先于 Active**：即使节点是活跃节点，禁用后也不会被使用
- **Disabled 独立于 Failed**：手动禁用和自动失败是两个独立状态
- **禁用不影响统计**：禁用的节点仍然保留所有统计数据

### 自动切换
当活跃节点被禁用时：
- 系统自动选择下一个最佳节点
- 按权重从高到低选择
- 跳过所有失败和禁用的节点

## 使用示例

### 场景 1：临时维护
```bash
# 1. 禁用需要维护的节点
curl -X POST http://localhost:8000/admin/api/nodes/disable \
  -H "Content-Type: application/json" \
  -d '{"id": "node-prod-1"}'

# 2. 维护完成后启用
curl -X POST http://localhost:8000/admin/api/nodes/enable \
  -H "Content-Type: application/json" \
  -d '{"id": "node-prod-1"}'
```

### 场景 2：成本控制
在管理页面：
1. 找到高成本节点（如海外节点）
2. 点击"禁用"按钮
3. 系统自动切换到其他节点
4. 需要时再点击"启用"恢复使用

### 场景 3：测试验证
```bash
# 禁用所有生产节点，只保留测试节点
curl -X POST http://localhost:8000/admin/api/nodes/disable \
  -H "Content-Type: application/json" \
  -d '{"id": "node-prod-1"}'

curl -X POST http://localhost:8000/admin/api/nodes/disable \
  -H "Content-Type: application/json" \
  -d '{"id": "node-prod-2"}'

# 验证完成后重新启用生产节点
```

## 注意事项

1. **至少保留一个可用节点**：不要禁用所有节点，否则服务不可用
2. **禁用不会删除数据**：禁用的节点保留所有配置和统计数据
3. **重启后保持状态**：禁用状态会持久化到数据库，重启后仍然生效
4. **活跃节点可以被禁用**：即使是当前活跃节点也可以禁用，系统会自动切换

## 版本信息
- 添加版本：v2.1.0
- 添加日期：2025-11-22
