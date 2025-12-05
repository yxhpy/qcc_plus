# 项目记忆文件

## 元信息
- **更新日期**: 2025-12-05
- **当前版本**: v1.8.4
- **最新功能**: 监控大屏显示熔断状态 + 透传上游状态码 + 健康检查历史显示所有来源
- **GitHub**: https://github.com/yxhpy/qcc_plus
- **Docker Hub**: https://hub.docker.com/r/yxhpy520/qcc_plus

## 项目概述
- **项目名称**: qcc_plus
- **项目类型**: Claude Code CLI 代理服务器
- **技术栈**: Go 1.21 + MySQL + React 18 + TypeScript + Vite
- **核心功能**:
  - Claude Code CLI 请求复刻与反向代理
  - 多租户账号隔离
  - 自动故障切换和探活（API/HEAD/CLI 三种健康检查）
  - React SPA 管理界面
  - MySQL 持久化配置

## 快速启动
```bash
# CLI 模式
go run ./cmd/cccli "hi"

# 代理服务器
UPSTREAM_BASE_URL=https://api.anthropic.com \
UPSTREAM_API_KEY=sk-ant-your-key \
go run ./cmd/cccli proxy

# Docker 部署
docker compose up -d
```

**默认凭证（仅内存模式）**：
- 管理员：`admin` / `admin123`
- 默认账号：`default` / `default123`
- 管理界面：http://localhost:8000/admin
- **生产环境必须修改默认密码与密钥！**

## 环境变量速查

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `LISTEN_ADDR` | 代理监听地址 | :8000 |
| `UPSTREAM_BASE_URL` | 上游 API 地址 | https://api.anthropic.com |
| `UPSTREAM_API_KEY` | 默认上游 API Key | - |
| `PROXY_RETRY_MAX` | 重试次数 | 3 |
| `PROXY_FAIL_THRESHOLD` | 失败阈值 | 3 |
| `PROXY_HEALTH_INTERVAL_SEC` | 探活间隔（秒） | 30 |
| `PROXY_MYSQL_DSN` | MySQL 连接 | - |
| `ADMIN_API_KEY` | 管理员密钥 | admin |

## 测试环境默认参数（`docker-compose.test.yml`，与 prod 保持一致）
- 重试与超时：`PROXY_RETRY_MAX/RETRY_MAX_ATTEMPTS=3`；单次超时 12/6/3s，`RETRY_TOTAL_TIMEOUT_SEC=25`，`RETRY_ON_STATUS=502,503,504`
- 健康检查：默认 `PROXY_HEALTH_CHECK_MODE=cli`；`PROXY_FAIL_THRESHOLD=3`，`PROXY_HEALTH_INTERVAL_SEC=30`，`HEALTH_ALL_INTERVAL_MIN=10`，`HEALTH_CHECK_CONCURRENCY=2`，`HEALTH_CHECK_CONCURRENCY_CLI=1`
- 熔断：窗口 120s，失败率 0.8，连续失败 10 次，冷却 60s，半开探测 5 次
- 预热：`WARMUP_ATTEMPTS=1`，`WARMUP_TIMEOUT_MS=10000`
- 传输层超时：TLS 握手 10s，响应头 30s

## 项目特异规则

| 规则 | 说明 |
|------|------|
| Builder 模式 | 代理服务器使用 Builder 模式，参考 `internal/proxy/` |
| 节点权重 | 权重值越小优先级越高（1 > 2 > 3）；事件驱动切换 |
| 时间格式 | 使用 `timeutil.FormatBeijingTime()`，北京时间 |
| UI 密度 | 单行紧凑显示；字体 12-14px；padding/gap 6-10px |

详细编码规范见 @docs/claude/coding-standards.md

## Git 分支策略

| 分支 | 用途 |
|------|------|
| `test` | **日常开发**（编写代码前必须确认在此分支） |
| `main` | 正式发布（打 tag） |
| `prod` | 生产部署 |

**发布**：`git tag vX.Y.Z && git push origin vX.Y.Z`（GoReleaser 自动化）

详细流程见 @docs/claude/git-workflow.md 和 @docs/claude/release-policy.md

## 测试部署流程

### 自动部署到测试环境

```bash
# 1. 确认在 test 分支
git checkout test

# 2. 提交代码
git add .
git commit -m "fix: 描述修复内容"

# 3. 推送触发自动部署
git push origin test

# GitHub Actions 会自动：
# - 构建前端
# - 构建 Docker 镜像
# - 部署到测试服务器（端口 8001）
# - 执行健康检查
```

### 连接测试服务器

**配置文件**：
- 复制 `.deploy-config.example` 为 `.deploy-config`（已加入 .gitignore）
- 填入服务器信息（见私有文档）

**私有配置**：
- 详见 `docs/claude/deployment-private.md`（仅本地，已加入 .gitignore）
- 包含服务器 IP、SSH 密钥、快速命令等

**快速命令模板**：
```bash
# 查看日志
ssh -i <ssh-key> <user>@<server> "docker logs <container> -f"

# 查看容器状态
ssh -i <ssh-key> <user>@<server> "docker ps -a"

# 重启服务
ssh -i <ssh-key> <user>@<server> "docker restart <container>"
```

**注意**：
- ⚠️ 测试服务器信息是私有的，不要提交到公开仓库
- ✅ 使用 `.deploy-config` 存储本地配置
- ✅ 使用 SSH 密钥而非密码认证

## 任务执行速查

1. 理解需求 → 2. 查阅文档 → 3. 分析设计 → 4. **使用 Codex Skill 编写代码** → 5. 测试验证 → 6. 更新文档

**Codex Skill 强制规则**：
- 模型：`gpt-5.1-codex-max`
- reasoning effort：`high`
- 使用临时文件避免 Shell 转义：`cat .codex_prompt.txt | codex exec ...`

详细流程见 @docs/claude/task-lifecycle.md

## 调试入口

| 问题类型 | 快速检查 |
|----------|----------|
| 400 错误 | `NO_WARMUP=1`、`MINIMAL_SYSTEM=1` |
| 代理连接失败 | 检查 `UPSTREAM_BASE_URL`、网络连通性 |
| MySQL 连接 | 检查 `PROXY_MYSQL_DSN` 格式 |
| CI/CD 超时 | 查看 `docker logs`，参考 `docs/ci-cd-troubleshooting.md` |

详细排查见 @docs/claude/debug-playbook.md

## 记忆更新规范

**遇到踩坑点时立即记录**，防止重复犯错：

1. **快速记录**：编辑 @docs/claude/lessons-learned.md
2. **格式**：`[日期] 问题` → 现象/原因/解决/预防
3. **分类**：代码类、配置类、部署类
4. **同步**：重要规则同步到对应的原子文档

踩坑记录见 @docs/claude/lessons-learned.md

## 文档索引

### Claude 专用文档
- @docs/claude/coding-standards.md - 编码规范
- @docs/claude/task-lifecycle.md - 任务执行流程
- @docs/claude/git-workflow.md - Git 工作流
- @docs/claude/release-policy.md - 版本发布规范
- @docs/claude/debug-playbook.md - 调试排查手册
- @docs/claude/lessons-learned.md - 踩坑记录
- @docs/claude/deployment-private.md - 🔒 私有部署配置（仅本地，已 ignore）

### 项目文档
- @README.md - 项目主页
- @CHANGELOG.md - 版本历史
- @docs/README.md - 完整文档索引
- @docs/multi-tenant-architecture.md - 多租户架构
- @docs/quick-start-multi-tenant.md - 多租户快速开始
- @docs/health_check_mechanism.md - 健康检查机制
- @docs/goreleaser-guide.md - GoReleaser 指南
- @docs/release-workflow.md - 发布流程详解
- @docs/ci-cd-troubleshooting.md - CI/CD 故障排查

### 前端文档
- @docs/frontend-tech-stack.md - 管理界面技术栈
- @frontend/README.md - 前端开发指南

### 官网文档
- @docs/website-README.md - 官网文档总览
- @docs/website-design-concept.md - 设计概念
- @docs/website-technical-spec.md - 技术规格

## 版本历史

详见 @CHANGELOG.md
