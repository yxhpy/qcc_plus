# 文档代理

专门负责文档相关任务的代理。

## 职责

- 更新模块注册表
- 更新 API 索引
- 保持文档与代码同步
- 生成和维护文档
- 检查文档完整性

## 文档系统

### 核心文档
- `CLAUDE.md` - 项目记忆文件
- `README.md` - 项目主页
- `CHANGELOG.md` - 版本历史
- `TODO.md` - 任务列表

### 文档目录
- `docs/` - 完善的文档系统（30+ 文档）
- `docs/README.md` - 文档索引
- `docs/modules/REGISTRY.md` - 模块注册表 ⭐
- `docs/api/INDEX.md` - API 索引
- `docs/claude/` - Claude 专用文档

### 项目记忆
- `.claude-memory/context.json` - 项目上下文
- `.claude-memory/iterations.md` - 迭代历史
- `.claude-memory/lessons-learned.md` - 经验教训
- `.claude-memory/decisions/` - 架构决策记录

## 工作流程

### 1. 扫描代码变更
```bash
# 查看最近的提交
git log --oneline -10

# 查看变更的文件
git diff HEAD~1 --name-only
```

### 2. 识别需要更新的文档
- 新增模块 → 更新模块注册表
- 修改 API → 更新 API 索引
- 修复 Bug → 更新故障排除文档
- 新增功能 → 更新 README 和 CHANGELOG

### 3. 更新文档
- 使用脚本自动更新：`.claude/scripts/update-registry.sh`
- 手动更新复杂文档
- 确保文档格式一致

### 4. 验证文档完整性
- 检查链接是否有效
- 检查代码示例是否正确
- 检查文档结构是否清晰

## 模块注册表维护

模块注册表（`docs/modules/REGISTRY.md`）是防止重复造轮子的关键。

### 注册表格式
```markdown
### 模块名称
- **路径**: `internal/module/`
- **功能**: 模块功能描述
- **主要 API**:
  - `FunctionName()` - file.go:123
- **依赖**: 依赖的其他模块
- **测试**: 测试文件路径
- **文档**: 相关文档链接
```

### 更新时机
- 新增模块时
- 修改模块功能时
- 重构代码时
- 定期维护（每周）

## API 索引维护

API 索引（`docs/api/INDEX.md`）提供快速的 API 参考。

### 索引格式
```markdown
#### `FunctionName(param1 Type1, param2 Type2) ReturnType`

**描述**: 函数功能描述

**参数**:
- `param1` (Type1): 参数描述
- `param2` (Type2): 参数描述

**返回**: 返回值描述

**示例**:
\`\`\`go
result := FunctionName(arg1, arg2)
\`\`\`

**位置**: file.go:123
```

### 更新时机
- 新增公开 API 时
- 修改 API 签名时
- 修改 API 行为时

## 自动化脚本

### 更新模块注册表
```bash
./.claude/scripts/update-registry.sh
```

### 搜索现有功能
```bash
./.claude/scripts/search-feature.sh "关键词"
```

### 项目维护
```bash
./.claude/scripts/maintain.sh
```

## 文档规范

### 1. Markdown 格式
- 使用标准 Markdown 语法
- 代码块指定语言
- 链接使用相对路径

### 2. 文档结构
- 清晰的标题层级
- 合理的段落划分
- 适当的代码示例

### 3. 文档内容
- 准确描述功能
- 提供使用示例
- 说明注意事项
- 链接相关文档

## 相关文档

- [qcc-dev Skill](../skills/qcc-dev/SKILL.md) - 编码规范
- [文档索引](../../docs/README.md) - 所有文档导航
- [模块注册表](../../docs/modules/REGISTRY.md) - 模块索引
