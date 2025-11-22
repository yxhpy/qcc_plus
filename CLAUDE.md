# 本文件为项目的记忆文件
- 必须保证本文件简洁、准确，并且保证文件实时更新

## 最后更新
- **更新日期**: 2025-11-22
- **更新人**: Claude Code
- **版本**: v3.1.0

## 项目概述
- **项目名称**: qcc_plus
- **项目类型**: Claude Code CLI 代理服务器
- **技术栈**:
  - **后端**: Go 1.21, MySQL, Docker
  - **前端**: React 18, TypeScript, Vite, Chart.js
- **主要功能**:
  - Claude Code CLI 请求复刻
  - 反向代理服务器（端口转发）
  - **多租户账号隔离**（v3.0 新增）
  - **工具定义自动清理**（v3.0.1 新增）
  - **React SPA 管理界面**（v3.1 新增）
  - **事件驱动节点切换**（v3.1.0 新增，显著提升性能）
  - 多节点管理（Web 管理页面）
  - 自动故障切换和探活
  - MySQL 持久化配置

## 文件结构
```
qcc_plus/
├── cmd/cccli/          # 入口 main，支持消息模式与 proxy 转发模式
├── internal/
│   ├── client/         # 请求构造、预热、SSE 流读取等核心逻辑
│   ├── proxy/          # Builder 模式的反向代理服务器（SPA 服务器）
│   └── store/          # 数据存储层
├── frontend/           # React 前端源码（开发）
│   ├── src/            # TypeScript/React 源码
│   ├── dist/           # 构建输出（Git 忽略）
│   └── package.json
├── web/                # Go embed 前端资源（生产）
│   ├── embed.go        # Embed 声明
│   └── dist/           # 前端构建产物（从 frontend/dist 复制）
├── cccli/              # 系统 prompt 模板和工具定义（embed）
├── scripts/            # 部署脚本（包括前端构建脚本）
├── .docker/            # Docker 相关配置
└── docs/               # 项目文档（包括前端技术栈说明）
```

## 快速启动
```bash
# 直接运行 CLI
go run ./cmd/cccli "hi"

# 启动代理服务器（默认多租户，使用默认凭证）
UPSTREAM_BASE_URL=https://api.anthropic.com \
UPSTREAM_API_KEY=sk-ant-your-key \
go run ./cmd/cccli proxy

# 启动时会输出：
# - Admin API Key: admin
# - Account 'default': proxy_api_key=default-proxy-key
# - 管理界面: http://localhost:8000/admin?admin_key=admin

# 使用默认账号测试
curl http://localhost:8000/v1/messages \
  -H "x-api-key: default-proxy-key" \
  -d '{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":"hi"}],"max_tokens":100}'

# Docker 部署
docker compose up -d
```

## 环境变量
| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| ANTHROPIC_AUTH_TOKEN | API Token（必须） | - |
| ANTHROPIC_BASE_URL | API 地址 | https://api.anthropic.com |
| MODEL | 主模型 | claude-sonnet-4-5-20250929 |
| WARMUP_MODEL | 预热模型 | claude-haiku-4-5-20251001 |
| NO_WARMUP | 跳过预热 | 0 |
| MINIMAL_SYSTEM | 使用精简系统提示 | 1 |
| PROXY_RETRY_MAX | 重试次数 | 3 |
| PROXY_MYSQL_DSN | MySQL 连接 | - |
| **ADMIN_API_KEY** | 管理员密钥（多租户） | - |
| **DEFAULT_ACCOUNT_NAME** | 默认账号名称 | default |
| **DEFAULT_PROXY_API_KEY** | 默认代理 API Key | - |

## 多租户架构（默认启用）
系统默认以多租户模式运行：
- **账号隔离**：每个账号拥有独立的节点池和配置
- **路由逻辑**：根据请求头 `x-api-key` 自动路由到对应账号的节点
- **权限控制**：管理员可管理所有账号，普通账号只能管理自己的资源
- **默认凭证**：
  - 管理员密钥：`admin`（环境变量 `ADMIN_API_KEY`）
  - 默认账号：`default`，proxy_api_key 为 `default-proxy-key`
  - **⚠️ 生产环境必须修改默认凭证！**

详细文档：`docs/multi-tenant-architecture.md`、`docs/quick-start-multi-tenant.md`

## 文档导航索引
<navigation description="快速定位所需章节">
    <章节索引>
        <section name="Codex Skill 强制使用规则" tag="codex_mandatory">**重要**：所有代码任务必须使用 Codex Skill</section>
        <section name="任务启动通用流程" tag="task_startup_flow">接收任何任务时的标准启动流程</section>
        <section name="基本执行流程" tag="steps">标准任务执行流程</section>
        <section name="编码规范" tag="coding_standards">Go 语言编码规则</section>
        <section name="版本控制" tag="version_control">Git 工作流和提交规范</section>
        <section name="质量保证" tag="quality_assurance">代码审查和测试要求</section>
    </章节索引>
</navigation>

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
</debugging>
