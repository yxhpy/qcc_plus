# 本文件为项目的记忆文件
- 必须保证本文件简洁、准确，并且保证文件实时更新

## 最后更新
- **更新日期**: 2025-11-26
- **更新人**: Claude Code
- **当前版本**: v1.3.0
- **最新功能**: 监控大屏和分享功能
- **最新更新**: 实时监控大屏、健康检查历史时间线、共享监控页面、分离代理流量和健康检查指标
- **GitHub**: https://github.com/yxhpy/qcc_plus
- **Docker Hub**: https://hub.docker.com/r/yxhpy520/qcc_plus

## 项目概述
- **项目名称**: qcc_plus
- **项目类型**: Claude Code CLI 代理服务器
- **技术栈**:
  - **后端**: Go 1.21, MySQL, Docker
  - **前端**: React 18, TypeScript, Vite, Chart.js
- **主要功能**:
  - Claude Code CLI 请求复刻
  - 反向代理服务器（端口转发）
  - 多租户账号隔离
  - 工具定义自动清理
  - React SPA 管理界面
  - 事件驱动节点切换（仅在节点状态变化时重选）
  - 多节点管理（Web 管理页面）
  - 自动故障切换和探活（支持 API/HEAD/CLI 三种健康检查方式）⭐ 新增
  - MySQL 持久化配置

## 文件结构
```
qcc_plus/
├── cmd/cccli/          # 入口 main，支持消息模式与 proxy 转发模式
├── internal/
│   ├── client/         # 请求构造、预热、SSE 流读取等核心逻辑
│   ├── proxy/          # Builder 模式的反向代理服务器（SPA 服务器）
│   └── store/          # 数据存储层（MySQL）
├── frontend/           # React 前端源码（React 18 + TypeScript + Vite）
│   ├── src/            # TypeScript/React 源码
│   ├── dist/           # 构建输出（Git 忽略）
│   └── package.json
├── website/            # 官网源码（Next.js 14 + Three.js）
│   ├── app/            # Next.js App Router
│   ├── components/     # React 组件（3D、UI、动画）
│   ├── hooks/          # 自定义 Hooks
│   ├── lib/            # 工具库
│   └── public/         # 静态资源（模型、纹理、图片）
├── web/                # Go embed 前端资源（生产）
│   ├── embed.go        # Embed 声明
│   └── dist/           # 前端构建产物（从 frontend/dist 复制）
├── cccli/              # 系统 prompt 模板和工具定义（embed）
├── scripts/            # 部署脚本（前端构建、Docker 发布、官网初始化）
├── docs/               # 项目文档（包括前端技术栈、官网设计文档）
├── docker-compose.yml  # Docker Compose 配置
└── Dockerfile          # Docker 镜像构建文件
```

## 快速启动
```bash
# 直接运行 CLI
go run ./cmd/cccli "hi"

# 启动代理服务器（默认多租户，使用默认凭证）
UPSTREAM_BASE_URL=https://api.anthropic.com \
UPSTREAM_API_KEY=sk-ant-your-key \
go run ./cmd/cccli proxy

# 启动时会输出（内存模式）：
# - 管理员登录：username=admin password=admin123
# - 默认账号：username=default password=default123
# - 管理界面: http://localhost:8000/admin
# 持久化模式（配置 PROXY_MYSQL_DSN）不会自动创建默认账号，请登录后手动创建。

# 使用默认账号测试
curl http://localhost:8000/v1/messages \
  -H "x-api-key: default-proxy-key" \
  -d '{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":"hi"}],"max_tokens":100}'

# 仅在存在默认账号且 proxy_api_key 为 default-proxy-key 时可用；持久化模式需登录后自行创建账号与节点。

# Docker 部署
docker compose up -d
```

## 环境变量
| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| ANTHROPIC_AUTH_TOKEN | API Token（CLI 消息模式） | - |
| ANTHROPIC_BASE_URL | API 地址（CLI 消息模式） | https://api.anthropic.com |
| MODEL | 主模型 | claude-sonnet-4-5-20250929 |
| WARMUP_MODEL | 预热模型 | claude-haiku-4-5-20251001 |
| NO_WARMUP | 跳过预热 | 0 |
| MINIMAL_SYSTEM | 使用精简系统提示 | 1 |
| **LISTEN_ADDR** | 代理监听地址 | :8000 |
| **UPSTREAM_BASE_URL** | 上游 API 地址 | https://api.anthropic.com |
| **UPSTREAM_API_KEY** | 默认上游 API Key | - |
| **UPSTREAM_NAME** | 默认节点名称 | default |
| **PROXY_RETRY_MAX** | 重试次数 | 3 |
| **PROXY_FAIL_THRESHOLD** | 失败阈值（连续失败多少次标记失败） | 3 |
| **PROXY_HEALTH_INTERVAL_SEC** | 探活间隔（秒） | 30 |
| **PROXY_MYSQL_DSN** | MySQL 连接 | - |
| **ADMIN_API_KEY** | 管理员密钥（服务端配置，非登录口令） | admin ⚠️ |
| **DEFAULT_ACCOUNT_NAME** | 默认账号名称（仅内存模式自动创建） | default |
| **DEFAULT_PROXY_API_KEY** | 默认代理 API Key（仅内存模式自动创建） | default-proxy-key ⚠️ |
| **CF_API_TOKEN** | Cloudflare API Token（隧道功能） | - |
| **TUNNEL_SUBDOMAIN** | 隧道子域名 | - |
| **TUNNEL_ZONE** | Cloudflare Zone（域名） | - |
| **TUNNEL_ENABLED** | 启用隧道功能 | false |

⚠️ **安全警告**：生产环境必须修改 `ADMIN_API_KEY` 和 `DEFAULT_PROXY_API_KEY`！

管理界面与管理 API 通过 `/login` 登录获得的 `session_token` Cookie 认证，不再使用 `x-admin-key` 头。

## 多租户架构（默认启用）
系统默认以多租户模式运行：
- **账号隔离**：每个账号拥有独立的节点池和配置
- **路由逻辑**：根据请求头 `x-api-key` 自动路由到对应账号的节点
- **权限控制**：管理员可管理所有账号，普通账号只能管理自己的资源
- **默认凭证（仅内存模式自动创建）**：
  - 管理员登录：username `admin` / password `admin123`
  - 默认账号：username `default` / password `default123`
  - 持久化模式不会自动创建默认账号，请登录后手动创建
  - **⚠️ 生产环境必须修改默认密码与密钥！**

详细文档：`docs/multi-tenant-architecture.md`、`docs/quick-start-multi-tenant.md`

## 文档导航

### 主文档
- **[README.md](README.md)** - 项目主页，快速开始和环境变量配置
- **[CHANGELOG.md](CHANGELOG.md)** - 版本更新日志
- **[docs/README.md](docs/README.md)** - 完整文档索引和导航
- **[CLAUDE.md](CLAUDE.md)** - 项目记忆文件（本文件）

### 后端文档
- **[docs/multi-tenant-architecture.md](docs/multi-tenant-architecture.md)** - 多租户架构设计
- **[docs/quick-start-multi-tenant.md](docs/quick-start-multi-tenant.md)** - 多租户快速开始
- **[docs/cloudflare-tunnel.md](docs/cloudflare-tunnel.md)** - Cloudflare Tunnel 集成指南
- **[docs/health_check_mechanism.md](docs/health_check_mechanism.md)** - 健康检查机制
- **[docs/release-workflow.md](docs/release-workflow.md)** - 发布流程最佳实践 ⭐ 必读
- **[docs/goreleaser-guide.md](docs/goreleaser-guide.md)** - GoReleaser 自动化发布指南
- **[docs/docker-hub-publish.md](docs/docker-hub-publish.md)** - Docker Hub 发布流程（手动模式，已弃用）
- **[docs/ci-cd-troubleshooting.md](docs/ci-cd-troubleshooting.md)** - CI/CD 部署故障排查指南

### 前端文档
- **[docs/frontend-tech-stack.md](docs/frontend-tech-stack.md)** - 管理界面技术栈和开发流程
- **[frontend/README.md](frontend/README.md)** - 管理界面开发指南

### 官网文档（新增）
- **[docs/website-README.md](docs/website-README.md)** - 官网文档总览和导航
- **[docs/website-design-concept.md](docs/website-design-concept.md)** - 设计概念与创新点
- **[docs/website-technical-spec.md](docs/website-technical-spec.md)** - 技术实现规格
- **[docs/website-implementation-roadmap.md](docs/website-implementation-roadmap.md)** - 6周实现路线图
- **[scripts/init-website.sh](scripts/init-website.sh)** - 官网项目初始化脚本

## 任务启动通用流程
<task_startup_flow description="接收到任何任务时的标准启动流程">
    <step_1 name="理解需求">理解用户需求的核心目标和约束条件</step_1>
    <step_2 name="查阅相关文档">根据任务类型查阅对应文档和代码</step_2>
    <step_3 name="检查依赖">确认前置条件和依赖项</step_3>
    <step_4 name="执行任务">按照基本执行流程完成任务</step_4>
</task_startup_flow>

## Codex Skill 强制使用规则
<codex_mandatory description="所有代码相关任务必须使用 Codex Skill">
    <rule>**强制规则**：所有代码编写、代码解析、代码分析任务必须使用 Codex Skill</rule>
    <rule>模型固定为 `gpt-5.1-codex-max`，reasoning effort 固定为 `high`</rule>
    <applicable_tasks>
        - 新功能代码编写
        - 代码重构和优化
        - Bug 修复
        - 代码审查和分析
        - 代码解释和理解
        - 测试代码编写
    </applicable_tasks>
    <usage>使用 Skill 工具调用 codex skill，或直接运行 codex exec 命令</usage>

    <best_practices description="Codex Skill 最佳实践">
        <practice name="避免 Shell 转义问题">
            <step>1. 将 prompt 内容写入临时文件（如 .codex_prompt.txt）</step>
            <step>2. 使用 cat 管道方式调用：cat .codex_prompt.txt | codex exec ...</step>
            <step>3. 任务完成后删除临时文件</step>
            <example>
                # 写入 prompt
                Write file: .codex_prompt.txt

                # 执行 codex
                cat .codex_prompt.txt | codex exec --model gpt-5.1-codex-max --config model_reasoning_effort=high --sandbox workspace-write --full-auto --skip-git-repo-check 2>/dev/null

                # 删除临时文件
                rm .codex_prompt.txt
            </example>
        </practice>
        <practice name="必要参数">
            <param>--model gpt-5.1-codex-max（必须）</param>
            <param>--config model_reasoning_effort=high（必须）</param>
            <param>--skip-git-repo-check（必须）</param>
            <param>--sandbox workspace-write（写文件时）或 read-only（仅分析时）</param>
            <param>--full-auto（自动执行，无需确认）</param>
            <param>2>/dev/null（隐藏 thinking tokens）</param>
        </practice>
        <practice name="Resume 继续会话">
            <step>使用 echo 管道方式继续：echo "new prompt" | codex exec --skip-git-repo-check resume --last 2>/dev/null</step>
            <step>Resume 时不需要指定 model 和 reasoning effort，会继承原会话设置</step>
        </practice>
    </best_practices>
</codex_mandatory>

## 基本执行流程
<steps description="标准任务执行流程">
    <step_1>
        <action>理解需求意图和目标</action>
        <detail>理解用户需求的核心目标和约束条件</detail>
    </step_1>
    <step_2>
        <action>分析和设计</action>
        <detail>分析需求，设计实现方案，必要时进行技术验证</detail>
    </step_2>
    <step_3>
        <action>编写代码（使用 Codex Skill）</action>
        <detail>**必须使用 Codex Skill** 完成代码实现，模型 gpt-5-codex-max，reasoning effort high</detail>
    </step_3>
    <step_4>
        <action>测试验证</action>
        <detail>编写测试用例，使用真实数据验证功能正确性</detail>
    </step_4>
    <step_5>
        <action>更新文档</action>
        <detail>更新相关文档，保持文档与代码一致</detail>
    </step_5>
</steps>

## 必须要有如下文件夹
<folder_structure description="项目文件夹结构规范">
    <folder_item path="verify/[功能名]/verify_*.go" description="技术验证代码，验证通过后标记为 _pass"/>
    <folder_item path="tests/[功能名]/test_*.go" description="测试代码，测试通过后标记为 _pass"/>
    <folder_item path="debugs/[问题名]/debug_*.go" description="调试代码，调试完成后标记为 _pass"/>
    <folder_item path="docs/*.md" description="项目文档"/>
    <folder_item path="tasks/[需求名]/story_*.md" description="任务文档，完成后标记为 _Y"/>
</folder_structure>

## 编码规范
<coding_standards description="Go 语言编码规范">
    <general_rules>
        <rule>遵循 Go 官方代码风格（gofmt）</rule>
        <rule>使用有意义的变量和函数命名</rule>
        <rule>函数保持单一职责，避免过长</rule>
        <rule>正确处理错误，不要忽略 error 返回值</rule>
        <rule>使用 context 进行超时和取消控制</rule>
        <rule>避免全局变量，使用依赖注入</rule>
    </general_rules>

    <project_rules description="项目特异性规则">
        <project_rules_item title="Builder 模式">代理服务器使用 Builder 模式构建，参考 internal/proxy/</project_rules_item>
        <project_rules_item title="环境变量配置">所有配置通过环境变量注入，参考 .env.example</project_rules_item>
        <project_rules_item title="MySQL 持久化">节点配置持久化到 MySQL，设置 PROXY_MYSQL_DSN 启用</project_rules_item>
        <project_rules_item title="SSE 流处理">SSE 流读取逻辑在 internal/client/ 中实现</project_rules_item>
        <project_rules_item title="请求指纹复刻">保持与官方 CLI 一致的请求头和参数</project_rules_item>
        <project_rules_item title="节点权重与切换">权重值越小优先级越高（1 > 2 > 3）；使用事件驱动切换，仅在节点状态变化时触发重选，避免请求路径扫描</project_rules_item>
        <project_rules_item title="时间格式统一">后端所有返回给前端显示的时间必须使用 `timeutil.FormatBeijingTime()`，输出格式为 `2006年01月02日 15时04分05秒`（北京时间 UTC+8）</project_rules_item>
        <project_rules_item title="UI高信息密度">所有页面必须保持高信息密度：单行紧凑显示优于网格布局；字体 12-14px；padding/gap 6-10px；避免滚动条；主要信息突出、次要信息用小字/括号弱化</project_rules_item>
    </project_rules>

    <error_handling>
        <rule>所有外部调用（HTTP、数据库）必须有超时控制</rule>
        <rule>错误信息要有足够上下文，便于排查</rule>
        <rule>使用 errors.Wrap/Wrapf 包装底层错误</rule>
        <rule>在边界层（HTTP handler）统一处理和记录错误</rule>
    </error_handling>
</coding_standards>

## 版本控制
<version_control description="Git 工作流规范">
    <branch_strategy description="分支策略（强制）">
        <rule>**强制规则**：所有开发工作必须在 `test` 分支进行，编写代码前必须确认当前分支</rule>
        <branches>
            <branch name="test" purpose="日常开发">✅ 在这里开发，推送后自动部署到测试环境（端口 8001）</branch>
            <branch name="main" purpose="正式发布">合并测试通过的代码，用于打 tag 发布版本</branch>
            <branch name="prod" purpose="生产部署">部署到生产服务器（端口 8000）</branch>
        </branches>
        <workflow>
            <step>1. 开发：git checkout test → 编写代码 → git push origin test</step>
            <step>2. 发布：git checkout main → git merge test → git tag vX.Y.Z → git push origin vX.Y.Z</step>
            <step>3. 部署：git checkout prod → git merge main → git push origin prod</step>
        </workflow>
        <pre_coding_checklist description="编写代码前必须执行">
            <check>git branch --show-current（确认在 test 分支）</check>
            <check>如不在 test 分支，执行 git checkout test</check>
        </pre_coding_checklist>
    </branch_strategy>
    <commit_format>[类型] 简短描述</commit_format>
    <commit_types>
        <type name="feat">新功能</type>
        <type name="fix">Bug 修复</type>
        <type name="docs">文档更新</type>
        <type name="refactor">代码重构</type>
        <type name="test">测试相关</type>
        <type name="chore">构建/工具</type>
    </commit_types>
</version_control>

## 质量保证
<quality_assurance description="代码质量要求">
    <testing>
        <rule>核心业务逻辑必须有单元测试</rule>
        <rule>使用真实数据测试，避免过度 mock</rule>
        <rule>测试边界条件和错误场景</rule>
        <rule>使用 go test -race 检测竞态条件</rule>
    </testing>
    <code_review>
        <rule>所有合并到 main 的代码必须经过审查</rule>
        <rule>检查错误处理是否完善</rule>
        <rule>检查是否有资源泄漏（goroutine、文件句柄）</rule>
        <rule>检查并发安全性</rule>
    </code_review>
</quality_assurance>

## 安全规范
<security description="安全编码要求">
    <rule>API Token 等敏感信息通过环境变量注入，禁止硬编码</rule>
    <rule>所有外部输入必须验证和过滤</rule>
    <rule>使用 HTTPS 进行外部通信</rule>
    <rule>日志中禁止输出敏感信息（token、密码）</rule>
    <rule>数据库查询使用参数化，防止 SQL 注入</rule>
</security>

## 调试指南
<debugging description="常见问题调试">
    <issue name="400 错误">
        <solution>检查 USER_HASH 是否匹配账号</solution>
        <solution>尝试 NO_WARMUP=1 跳过预热</solution>
        <solution>确认使用精简系统提示 MINIMAL_SYSTEM=1</solution>
    </issue>
    <issue name="工具定义格式错误 (tools.*.custom)">
        <solution>v3.0.1+ 自动清理工具定义中的非标准字段（如 custom、input_examples）</solution>
        <solution>代理会自动移除 Anthropic API 不支持的字段，保留 name/description/input_schema</solution>
        <solution>如需查看清理日志，检查代理服务器输出</solution>
    </issue>
    <issue name="代理连接失败">
        <solution>检查 UPSTREAM_BASE_URL 配置</solution>
        <solution>确认网络连通性</solution>
        <solution>查看 PROXY_RETRY_MAX 重试配置</solution>
    </issue>
    <issue name="MySQL 连接问题">
        <solution>检查 PROXY_MYSQL_DSN 格式</solution>
        <solution>确认 MySQL 服务运行状态</solution>
        <solution>检查防火墙和端口配置</solution>
    </issue>
    <issue name="CI/CD 健康检查超时">
        <solution>详见 docs/ci-cd-troubleshooting.md</solution>
        <solution>v1.0.1+ 已增强健康检查：10s 初始等待 + 6 次重试</solution>
        <solution>检查服务器端口和防火墙配置</solution>
        <solution>查看部署日志: docker logs qcc_test-proxy-1</solution>
    </issue>
</debugging>

## 版本发布规范
<release_process description="GitHub Release 和 Docker Hub 发布流程">
    <version_scheme description="语义化版本规范">
        <rule>使用语义化版本号：vX.Y.Z</rule>
        <rule>X (主版本号)：不兼容的 API 变更</rule>
        <rule>Y (次版本号)：向后兼容的功能新增</rule>
        <rule>Z (修订号)：向后兼容的问题修正</rule>
        <example>v1.0.0 - 首个正式版本</example>
        <example>v1.1.0 - 新增功能（向后兼容）</example>
        <example>v1.1.1 - Bug 修复</example>
        <example>v2.0.0 - 重大更新（可能不兼容）</example>
    </version_scheme>

    <goreleaser_automation description="GoReleaser 自动化发布（推荐）">
        <overview>
            本项目已集成 GoReleaser，实现完全自动化的版本发布流程。
            发布新版本只需创建并推送 Git tag，所有其他步骤自动完成。
        </overview>

        <quick_release name="快速发布">
            <command>git tag v1.2.0</command>
            <command>git push origin v1.2.0</command>
            <note>GitHub Actions 会自动触发 GoReleaser，完成以下所有步骤：</note>
            <automated_steps>
                <step>构建多平台 Go 二进制（Linux/macOS/Windows，amd64/arm64）</step>
                <step>注入版本信息（version、git commit、build date）</step>
                <step>构建并推送 Docker 镜像（amd64 + arm64 multi-arch）</step>
                <step>生成分类 CHANGELOG（根据 commit message）</step>
                <step>创建 GitHub Release 并上传所有构建产物</step>
                <step>更新 Docker Hub 仓库信息</step>
            </automated_steps>
        </quick_release>

        <commit_convention name="Commit Message 规范">
            <rule>遵循 Conventional Commits 格式以自动生成高质量 CHANGELOG</rule>
            <format>type: description</format>
            <types>
                <type name="feat">新功能（触发 minor 版本升级）</type>
                <type name="fix">Bug 修复（触发 patch 版本升级）</type>
                <type name="feat!" or "fix!">重大变更（触发 major 版本升级）</type>
                <type name="docs">文档更新（不触发版本升级）</type>
                <type name="refactor">代码重构（不触发版本升级）</type>
                <type name="test">测试相关（不触发版本升级）</type>
                <type name="chore">构建/工具（不包含在 CHANGELOG）</type>
            </types>
            <examples>
                <example>feat: 添加健康检查 API 端点</example>
                <example>fix: 修复 Docker 容器健康检查超时</example>
                <example>feat!: 重构 API 接口，移除 v1 兼容性</example>
                <example>docs: 更新 GoReleaser 使用说明</example>
            </examples>
        </commit_convention>

        <github_secrets name="GitHub Secrets 配置">
            <secret name="DOCKER_USERNAME">Docker Hub 用户名（yxhpy520）</secret>
            <secret name="DOCKER_TOKEN">Docker Hub Personal Access Token</secret>
            <note>在 GitHub 仓库设置 → Secrets and variables → Actions 中配置</note>
        </github_secrets>

        <local_testing name="本地测试">
            <command desc="检查配置">goreleaser check</command>
            <command desc="构建测试（快照模式）">goreleaser build --snapshot --clean</command>
            <command desc="完整发布测试（不推送）">goreleaser release --snapshot --clean --skip=publish</command>
        </local_testing>

        <reference>详细文档: docs/goreleaser-guide.md</reference>
    </goreleaser_automation>

    <release_workflow description="完整发布流程（手动模式，已弃用）">
        <step_1 name="准备发布">
            <action>确保所有更改已提交到 main 分支</action>
            <action>更新 CHANGELOG.md：将 [Unreleased] 内容移至新版本，添加版本号和日期</action>
            <action>更新 CLAUDE.md 中的版本号和更新日期</action>
            <action>确认 README.md 和相关文档已更新</action>
            <action>提交更新：git commit -am "chore: 准备发布 vX.Y.Z"</action>
        </step_1>

        <step_2 name="创建 Git 标签">
            <command>git tag vX.Y.Z</command>
            <command>git push origin vX.Y.Z</command>
            <note>标签名称必须以 v 开头，如 v1.0.0</note>
        </step_2>

        <step_3 name="创建 GitHub Release">
            <command>gh release create vX.Y.Z --title "vX.Y.Z - 版本描述" --notes "发布说明"</command>
            <content_template>
                # qcc_plus vX.Y.Z

                ## 概述
                [简要描述本次发布的主要内容]

                ## 核心特性
                - [功能列表]

                ## 更新内容
                - [变更列表]

                ## 安装方式
                Docker: docker pull yxhpy520/qcc_plus:vX.Y.Z
                源码: git clone -b vX.Y.Z https://github.com/yxhpy/qcc_plus.git
            </content_template>
        </step_3>

        <step_4 name="发布到 Docker Hub">
            <prerequisite>确保已登录 Docker Hub: docker login</prerequisite>
            <prerequisite>确保 Docker Hub 仓库已创建: yxhpy520/qcc_plus</prerequisite>
            <command>./scripts/publish-docker.sh yxhpy520 vX.Y.Z</command>
            <note>脚本会自动构建镜像并推送 vX.Y.Z 和 latest 标签</note>
            <note>版本信息会通过 ldflags 自动注入到二进制文件中</note>
            <note>构建脚本自动获取 git commit、build date 并注入到 internal/version 包</note>
            <manual_steps>
                <step>docker build --build-arg VERSION=vX.Y.Z --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") -t yxhpy520/qcc_plus:vX.Y.Z .</step>
                <step>docker tag yxhpy520/qcc_plus:vX.Y.Z yxhpy520/qcc_plus:latest</step>
                <step>docker push yxhpy520/qcc_plus:vX.Y.Z</step>
                <step>docker push yxhpy520/qcc_plus:latest</step>
            </manual_steps>
            <dockerhub_info_update name="更新 Docker Hub 仓库信息">
                <important>每次发布新版本后必须更新 Docker Hub 仓库的 Short Description 和 Full Description</important>
                <method_1 name="使用自动化脚本（推荐）">
                    <prerequisite>需要 Docker Hub Personal Access Token</prerequisite>
                    <command>./scripts/update-dockerhub-info-v2.sh YOUR_DOCKERHUB_TOKEN</command>
                    <note>脚本会自动更新 Short Description 和 Full Description</note>
                    <note>Short Description 限制 100 字节（中文约 33 个字符）</note>
                    <note>当前使用: "Claude CLI 多租户代理 | 自动切换 | Web管理" (53 字节)</note>
                </method_1>
                <method_2 name="手动更新">
                    <step>访问 https://hub.docker.com/repository/docker/yxhpy520/qcc_plus/general</step>
                    <step>更新 Short Description: 复制 docker-hub-description-options.md 中推荐的描述</step>
                    <step>更新 Full Description: 复制 README.dockerhub.md 的完整内容</step>
                    <step>设置 Category: 选择 "Networking" 或 "Developer Tools"</step>
                    <step>点击 Update 保存所有更改</step>
                </method_2>
                <verification>
                    <check>访问 https://hub.docker.com/r/yxhpy520/qcc_plus 验证更新</check>
                    <check>确认 Short Description 正确显示</check>
                    <check>确认 Full Description Markdown 格式正确</check>
                    <check>确认 Category 已设置</check>
                </verification>
            </dockerhub_info_update>
        </step_4>

        <step_5 name="验证发布">
            <check>GitHub Release: gh release view vX.Y.Z</check>
            <check>Docker 镜像: docker pull yxhpy520/qcc_plus:vX.Y.Z</check>
            <check>版本信息: curl http://localhost:8000/version（启动容器后验证版本信息正确）</check>
            <check>前端显示: 访问 http://localhost:8000/admin 查看侧边栏底部版本信息</check>
            <check>功能测试: 拉取镜像并运行基本功能测试</check>
        </step_5>

        <step_6 name="更新记忆文件">
            <action>更新 CLAUDE.md 中的"当前版本"字段</action>
            <action>记录此次发布的关键信息</action>
            <action>提交并推送更新</action>
        </step_6>
    </release_workflow>

    <important_notes description="发布注意事项">
        <note>首个正式版本从 v1.0.0 开始，不要使用过大的版本号</note>
        <note>Docker Hub 用户名是 yxhpy520（不是 yxhpy）</note>
        <note>latest 标签始终指向最新的稳定版本</note>
        <note>发布前必须确保代码已通过所有测试</note>
        <note>GitHub Release 应包含详细的更新说明和安装指南</note>
        <note>每次发布前必须更新 CHANGELOG.md，记录版本更新内容</note>
        <note>每次发布后立即更新 CLAUDE.md 记忆文件</note>
        <note>版本信息通过构建时 ldflags 注入，无需手动修改代码</note>
        <note>前端会自动从 /version API 获取版本信息并显示在侧边栏底部</note>
        <note>⚠️ 每次发布新版本后必须更新 Docker Hub 仓库信息（Short Description + Full Description）</note>
        <note>Docker Hub Short Description 限制 100 字节，中文字符需特别注意（1 个中文 = 3 字节）</note>
        <note>使用 scripts/update-dockerhub-info-v2.sh 脚本自动更新，或通过 Web 界面手动更新</note>
    </important_notes>

    <version_history description="版本发布历史">
        <release version="v1.3.0" date="2025-11-26">
            <description>监控大屏和分享功能</description>
            <highlights>
                - 实时监控大屏界面
                - 健康检查历史时间线
                - 共享监控页面和分享链接
                - 分离代理流量和健康检查指标
            </highlights>
            <github>https://github.com/yxhpy/qcc_plus/releases/tag/v1.3.0</github>
            <docker>yxhpy520/qcc_plus:v1.3.0</docker>
        </release>
        <release version="v1.2.0" date="2025-11-25">
            <description>节点拖拽排序和时间统一</description>
            <highlights>
                - 节点拖拽排序功能
                - 统一北京时间显示
            </highlights>
            <github>https://github.com/yxhpy/qcc_plus/releases/tag/v1.2.0</github>
            <docker>yxhpy520/qcc_plus:v1.2.0</docker>
        </release>
        <release version="v1.1.0" date="2025-11-24">
            <description>CLI 健康检查和通知系统</description>
            <highlights>
                - CLI 健康检查系统
                - 版本管理系统
                - 通知系统
                - CI/CD 自动化
            </highlights>
            <github>https://github.com/yxhpy/qcc_plus/releases/tag/v1.1.0</github>
            <docker>yxhpy520/qcc_plus:v1.1.0</docker>
        </release>
        <release version="v1.0.0" date="2025-11-23">
            <description>首个正式版本</description>
            <highlights>
                - 多租户架构支持
                - React Web 管理界面
                - Cloudflare Tunnel 集成
                - Docker 化部署支持
            </highlights>
            <github>https://github.com/yxhpy/qcc_plus/releases/tag/v1.0.0</github>
            <docker>yxhpy520/qcc_plus:v1.0.0</docker>
        </release>
    </version_history>
</release_process>
