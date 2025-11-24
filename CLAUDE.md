# 本文件为项目的记忆文件
- 必须保证本文件简洁、准确，并且保证文件实时更新

## 最后更新
- **更新日期**: 2025-11-24
- **更新人**: Claude Code
- **当前版本**: v1.0.0
- **最新功能**: 新增 CLI 健康检查方式（支持 Claude Code CLI 无头模式验证）
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
- **[docs/docker-hub-publish.md](docs/docker-hub-publish.md)** - Docker Hub 发布流程
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

    <release_workflow description="完整发布流程">
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
    </important_notes>

    <version_history description="版本发布历史">
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
