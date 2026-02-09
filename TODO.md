# 任务列表

## 说明

输入 **"开始"** 启动开发，AI 会自动读取本文件并执行任务。

## 升级后的首要任务

- [ ] 审查生成的 `.claude-memory/` 文件
- [ ] 审查生成的 `docs/modules/REGISTRY.md`
- [ ] 审查生成的 `docs/api/INDEX.md`
- [ ] 运行测试检查当前覆盖率: `go test -v -cover ./...`
- [ ] 补充测试达到 100% 覆盖率（优先级：proxy > store > notify > tunnel）
- [ ] 完善模块注册表中的功能描述
- [ ] 完善 API 索引中的代码示例

## 测试覆盖率提升计划

### 高优先级（必须 100% 覆盖）
- [ ] `internal/proxy/` - 核心代理逻辑
  - [ ] handler.go - 请求处理
  - [ ] node_manager.go - 节点管理
  - [ ] health.go - 健康检查
  - [ ] circuit_breaker.go - 熔断器
- [ ] `internal/client/` - API 客户端（当前 18.5%）
  - [ ] 补充边界测试
  - [ ] 补充错误处理测试
- [ ] `internal/store/` - 数据持久化（当前 0%）
  - [ ] account.go - 账号存储
  - [ ] node.go - 节点存储
  - [ ] metrics.go - 指标存储

### 中优先级（建议 80%+ 覆盖）
- [ ] `internal/notify/` - 通知系统（当前 0%）
- [ ] `internal/tunnel/` - 隧道管理（当前 0%）

### 低优先级（建议 60%+ 覆盖）
- [ ] `internal/timeutil/` - 工具函数
- [ ] `internal/version/` - 版本信息

## 文档完善

- [ ] 补充模块注册表中的功能描述
- [ ] 补充 API 索引中的代码示例
- [ ] 更新 CLAUDE.md 引用新的文档结构
- [ ] 添加使用示例到各个模块文档

## 自动化改进

- [ ] 测试 `.claude/scripts/search-feature.sh` 脚本
- [ ] 测试 `.claude/scripts/update-registry.sh` 脚本
- [ ] 测试 `.claude/scripts/maintain.sh` 脚本
- [ ] 考虑启用 Git hooks（`.claude/hooks/settings.json`）

## 原有任务

（从 GitHub Issues 或其他地方迁移）

---

## 快速命令

### 搜索现有功能
```bash
./.claude/scripts/search-feature.sh "关键词"
```

### 更新模块注册表
```bash
./.claude/scripts/update-registry.sh
```

### 运行测试
```bash
go test -v -cover ./...
```

### 生成覆盖率报告
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 项目维护
```bash
./.claude/scripts/maintain.sh
```

---

## 使用技巧

1. **防止重复造轮子**：开发前先运行 `search-feature.sh` 搜索现有功能
2. **查看模块注册表**：阅读 `docs/modules/REGISTRY.md` 了解现有模块
3. **查看已知问题**：阅读 `docs/claude/lessons-learned.md` 避免重复踩坑
4. **启动开发**：输入 **"开始"** 让 AI 自动执行任务
