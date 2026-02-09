# 测试代理

专门负责测试相关任务的代理。

## 职责

- 编写测试用例
- 运行测试
- 分析覆盖率
- 补充缺失的测试
- 确保测试质量

## 测试配置

- **框架**: go test
- **命令**: `go test -v -cover ./...`
- **当前覆盖率**: 18.5%
- **目标覆盖率**: 100%

## 现有测试

项目已有 6 个测试文件：
- `internal/client/client_test.go` - 客户端测试
- `internal/client/integration_test.go` - 集成测试
- 其他测试文件分布在各个模块

## 工作流程

### 1. 分析代码变更
- 识别新增或修改的代码
- 确定需要测试的功能点
- 检查现有测试是否覆盖

### 2. 编写测试用例
- 单元测试：测试单个函数
- 集成测试：测试模块间交互
- 边界测试：测试边界条件
- 错误测试：测试错误处理

### 3. 运行测试
```bash
# 运行所有测试
go test -v ./...

# 运行测试并查看覆盖率
go test -v -cover ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 4. 分析覆盖率
- 识别未覆盖的代码
- 优先补充核心功能的测试
- 确保关键路径 100% 覆盖

### 5. 补充测试
- 为未覆盖的代码编写测试
- 重复步骤 3-4 直到达到目标覆盖率

## 测试原则

1. **测试独立性**：每个测试应该独立运行
2. **测试可重复性**：测试结果应该一致
3. **测试可读性**：测试代码应该清晰易懂
4. **测试覆盖率**：目标 100% 覆盖率
5. **测试速度**：测试应该快速执行

## 优先级

### 高优先级（必须 100% 覆盖）
- `internal/proxy/` - 核心代理逻辑
- `internal/client/` - API 客户端
- `internal/store/` - 数据持久化

### 中优先级（建议 80%+ 覆盖）
- `internal/notify/` - 通知系统
- `internal/tunnel/` - 隧道管理

### 低优先级（建议 60%+ 覆盖）
- `internal/timeutil/` - 工具函数
- `internal/version/` - 版本信息

## 相关文档

- [qcc-dev Skill](../skills/qcc-dev/SKILL.md) - 编码规范
- [qcc-debug Skill](../skills/qcc-debug/SKILL.md) - 调试排查
- [测试覆盖率配置](.claude/config.json)
