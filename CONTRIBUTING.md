# 贡献指南

感谢你考虑为 qcc_plus 项目做出贡献！

## 如何贡献

### 报告问题

如果你发现了 bug 或有功能建议：

1. 在 [Issues](https://github.com/yxhpy/qcc_plus/issues) 中搜索，确认问题尚未被报告
2. 创建新的 Issue，详细描述：
   - Bug：复现步骤、预期行为、实际行为、环境信息
   - 功能建议：使用场景、预期效果、可能的实现方案

### 提交代码

#### 开发流程

1. **Fork 项目**
   ```bash
   # 在 GitHub 上 Fork 仓库
   git clone https://github.com/YOUR_USERNAME/qcc_plus.git
   cd qcc_plus
   ```

2. **创建功能分支**
   ```bash
   git checkout -b feature/your-feature-name
   # 或
   git checkout -b fix/your-bug-fix
   ```

3. **开发和测试**
   ```bash
   # 运行测试
   go test ./...

   # 构建前端（如果修改了前端）
   cd frontend
   npm install
   npm run build
   cd ..

   # 构建项目
   go build ./cmd/cccli
   ```

4. **提交代码**
   ```bash
   git add .
   git commit -m "feat: 添加新功能描述"
   # 或
   git commit -m "fix: 修复问题描述"
   ```

5. **推送并创建 Pull Request**
   ```bash
   git push origin feature/your-feature-name
   # 在 GitHub 上创建 Pull Request
   ```

#### 提交消息规范

使用语义化提交消息：

- `feat:` 新功能
- `fix:` Bug 修复
- `docs:` 文档更新
- `style:` 代码格式（不影响功能）
- `refactor:` 代码重构
- `test:` 测试相关
- `chore:` 构建/工具相关

示例：
```
feat: 添加节点健康检查超时配置

- 支持通过环境变量配置超时时间
- 添加相关单元测试
- 更新文档说明
```

#### 代码规范

**Go 代码**
- 遵循 `gofmt` 格式
- 使用 `go vet` 检查
- 添加必要的注释
- 编写单元测试（覆盖率 > 80%）

**前端代码**
- 遵循 ESLint 规则
- 使用 TypeScript 类型
- 组件保持简洁（< 300 行）
- 添加 PropTypes 或 TypeScript 类型

**文档**
- 使用中文编写
- 保持简洁准确
- 代码示例可运行
- 同步更新 CLAUDE.md

### Pull Request 检查清单

提交 PR 前确认：

- [ ] 代码通过所有测试：`go test ./...`
- [ ] 代码符合格式规范：`gofmt -w .`
- [ ] 添加了必要的测试
- [ ] 更新了相关文档
- [ ] 提交消息符合规范
- [ ] PR 描述清晰说明了改动内容

### 代码审查

- 维护者会尽快审查你的 PR
- 可能会提出修改建议
- 请及时响应评论和反馈
- 所有讨论解决后 PR 会被合并

## 开发环境设置

### 后端开发

**要求**
- Go 1.21+
- MySQL 8.0+（可选，用于持久化）

**设置**
```bash
# 安装依赖
go mod download

# 运行测试
go test ./...

# 本地运行
go run ./cmd/cccli proxy
```

### 前端开发

**要求**
- Node.js 18+
- npm 或 yarn

**设置**
```bash
cd frontend

# 安装依赖
npm install

# 开发模式（热重载）
npm run dev

# 构建
npm run build
```

### Docker 开发

```bash
# 构建镜像
docker build -t qcc_plus:dev .

# 运行容器
docker compose up -d
```

## 项目结构

```
qcc_plus/
├── cmd/cccli/              # 程序入口
├── internal/               # Go 核心业务逻辑
│   ├── client/             # Claude API 客户端
│   ├── proxy/              # 反向代理服务器
│   ├── store/              # 数据持久化（MySQL / SQLite）
│   ├── notify/             # 通知系统（微信等）
│   ├── tunnel/             # Cloudflare Tunnel 管理
│   ├── timeutil/           # 时间工具
│   └── version/            # 版本信息
├── frontend/               # React 18 前端
├── web/                    # Go embed 前端资源
├── cccli/                  # 系统 prompt 和工具定义
├── npm-packages/           # @qccplus/cli npm 分发包
├── website/                # 官网（Next.js）
├── scripts/                # 部署和构建脚本
├── tests/                  # 集成测试
├── docs/                   # 项目文档
└── .github/                # GitHub Actions & Issue 模板
```

## 测试

### 单元测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/proxy

# 查看覆盖率
go test -cover ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 集成测试

```bash
# 启动测试环境
docker compose -f docker-compose.test.yml up -d

# 运行集成测试
go test -tags=integration ./...

# 清理
docker compose -f docker-compose.test.yml down
```

## 发布流程

维护者负责发布新版本：

1. 更新版本号和 CHANGELOG
2. 创建 Git 标签：`git tag v1.x.x`
3. 推送标签：`git push origin v1.x.x`
4. 创建 GitHub Release
5. 发布 Docker 镜像：`./scripts/publish-docker.sh yxhpy520 v1.x.x`

## 获取帮助

- **文档**：查看 [docs/](./docs/) 目录
- **Issues**：在 [GitHub Issues](https://github.com/yxhpy/qcc_plus/issues) 提问
- **Discussions**：参与 [GitHub Discussions](https://github.com/yxhpy/qcc_plus/discussions)

## 行为准则

- 尊重所有贡献者
- 欢迎不同观点和建设性反馈
- 专注于项目改进，而非个人攻击
- 保持友好和专业的交流

## 许可证

贡献的代码将采用 [MIT License](./LICENSE) 发布。

---

再次感谢你的贡献！🎉
