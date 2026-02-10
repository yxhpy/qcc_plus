# 任务列表

## 说明

输入 **"开始"** 启动开发，AI 会自动读取本文件并执行任务。

## 待办任务

（当前无待办任务）

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
