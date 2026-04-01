# R001 需求分析：qcc_plus 文档整理与校准

## 1. 需求定性

**结论**：优化类需求（偏文档治理），同时带有一部分“文档错误修复”属性。

**判断依据**：
- 目标不是新增业务能力，也不是直接修复线上运行缺陷，而是统一整理现有文档体系。
- 当前仓库已出现多处文档与实现不一致，属于存量文档漂移治理。
- 根据当前扫描结果，至少存在以下已确认偏差：
  - `README.md` 头部版本徽标仍显示 `1.9.4`，与项目记忆中的 `v1.12.1`、`docs/README.md` 的 `v1.12.0` 不一致。
  - `docs/README.md` 与 `AGENTS.md` 引用了多个当前仓库不存在的路径，如 `.claude/scripts/search-feature.sh`、`.claude/scripts/update-registry.sh`、`.claude/agents/test-agent.md`、`docs/claude/task-lifecycle.md`、`docs/claude/debug-playbook.md`。
  - `docs/modules/REGISTRY.md` 中前端页面、覆盖率、模块说明已明显落后于当前代码；例如代码中实际存在 `RequestLogs`、`Pricing`、`ModelRecovery`、`ErrorPolicies` 等页面。

## 2. 涉及文件列表（预估）

以下为**高概率涉及**的文件，实际执行时应先做一次文档清单审计再最终确认：

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `docs/README.md`
- `docs/api/INDEX.md`
- `docs/modules/REGISTRY.md`
- `docs/claude/lessons-learned.md`
- `docs/quick-start-multi-tenant.md`
- `docs/multi-tenant-architecture.md`
- `docs/frontend-tech-stack.md`
- `docs/health_check_mechanism.md`
- `docs/monitoring-data-persistence.md`
- `docs/notification-system.md`
- `docs/cloudflare-tunnel.md`
- `docs/node-switch-optimization.md`
- `docs/cost-first-node-switching.md`
- `docs/goreleaser-guide.md`
- `docs/release-workflow.md`
- `docs/ci-cd-troubleshooting.md`
- `npm-packages/@qccplus/cli/README.md`

以下为**需要作为事实来源核对**的代码/配置目录：

- `internal/proxy/`
- `internal/store/`
- `internal/client/`
- `internal/notify/`
- `internal/tunnel/`
- `internal/timeutil/`
- `internal/version/`
- `internal/importer/`
- `frontend/src/App.tsx`
- `frontend/src/pages/`
- `scripts/`
- `cmd/`

补充说明：
- 当前仓库下 `docs/` 共 17 个文件，全部 Markdown 共 52 个文件，文档治理范围不算小。
- 本需求很可能还会顺带处理“删除失效引用”与“补充缺失索引说明”。

## 3. 依赖检查

### 3.1 与其他待开发需求的文件交叉

**当前结论：未发现明确的待开发需求文件。**

依据：
- `.codex-req/` 目录当前为空，本需求将是首个记录文件。
- 仓库内未发现明确的 `TODO.md` 或其他需求排期文件。

### 3.2 潜在文件交叉

虽然没有显式待开发需求，但该需求会与后续多数开发任务发生**高概率文件交叉**：

- `README.md`
- `AGENTS.md`
- `docs/README.md`
- `docs/modules/REGISTRY.md`
- `docs/api/INDEX.md`
- `CHANGELOG.md`

原因：
- 这些文件本身就是项目导航、模块说明、API 入口和版本信息的汇总点，任何后续功能迭代都可能继续修改它们。

### 3.3 逻辑依赖

该需求对以下事实源存在强依赖，必须以代码为准反向校正文档：

- 前端真实路由与页面：`frontend/src/App.tsx`、`frontend/src/pages/`
- 后端真实管理接口与代理接口：`internal/proxy/`
- 当前模块边界：`internal/*`
- 当前脚本能力：`scripts/`
- 当前版本与发布记录：`CHANGELOG.md`、`internal/version/`

额外发现：
- `AGENTS.md` 中记录的若干 `.claude/*` 自动化能力在当前仓库不存在，这会直接影响“开发工作流说明”的可信度，应视为本需求的一部分处理。

## 4. 实现方案概述（3-5 步）

1. 先做一轮“文档资产审计”。
   以 `README.md`、`docs/README.md`、`AGENTS.md` 为入口，列出所有文档、引用链接、脚本路径、代理/页面/API 声明，并标记失效项与待核对项。

2. 建立“文档事实源映射”。
   以代码为准，对照 `frontend/src/App.tsx`、`internal/proxy/`、`internal/*`、`scripts/`、`CHANGELOG.md`，确认真实页面、接口、模块、脚本、版本与部署流程。

3. 分层整理文档结构。
   明确“入口文档 / 架构文档 / 使用文档 / 运维发布文档 / Claude 专用文档 / 自动生成索引”的边界，删除失效引用，合并重复说明，补充缺失导航。

4. 逐份校正文案与示例。
   修正版本号、路径、命令、页面说明、API 列表、模块描述、部署说明，确保每个文档都能追溯到代码或现行脚本。

5. 做一次一致性回归检查。
   检查链接可达性、命令可执行性、目录是否存在、版本是否统一，并确认 `README.md`、`docs/README.md`、`AGENTS.md`、`REGISTRY.md`、`API 索引` 五个核心入口彼此一致。

## 5. 风险点

- **范围蔓延风险**：需求描述是“整理所有文档”，如果不先定义范围，容易从 `docs/` 扩散到 `README`、npm 包说明、发布说明、私有文档引用，工作量会失控。
- **以文档修正文档的风险**：现有文档之间已经互相冲突，不能拿某一份旧文档当事实源，必须回到代码与脚本本身。
- **隐性缺失风险**：当前已有多个被引用但不存在的文件/脚本，执行中可能继续发现更多“历史上曾存在但已删除”的文档入口。
- **自动生成声明失真风险**：`docs/api/INDEX.md`、`docs/modules/REGISTRY.md` 标注了“自动生成”，但当前仓库缺少对应 `.claude/scripts/*`，需要先确认是否仍有生成链路，否则容易改完又失真。
- **版本口径不统一风险**：当前至少存在 `1.9.4`、`1.12.0`、`1.12.1`、`dev` 多种版本表达；若不先定义“对外版本口径”，修订后仍可能继续冲突。
- **私有/外部文档边界风险**：如 `docs/claude/deployment-private.md` 和外部链接文档，公开索引中应保留到什么程度，需要先定规则。

## 6. 建议：接受/拒绝

**建议：接受**

**原因**：
- 当前文档漂移已经是事实，不是抽象担忧；至少已发现版本号、路径、页面清单、自动化说明四类不一致。
- 该需求不会改变业务逻辑，但会直接提升后续开发、排障、发布、接手维护的效率，属于高收益治理项。
- 这类问题越晚处理，后续需求越容易继续在错误文档基础上叠加，修复成本会上升。

**接受前建议补充一个边界定义**：
- 是否将 `README.md`、`AGENTS.md`、`docs/`、`npm-packages/@qccplus/cli/README.md` 全部纳入“所有文档”。
- 是否允许删除失效文档入口，还是仅做内容修正。

## 7. 预估优先级

**优先级：2**

说明：
- 它不是直接阻断运行的 P0/P1 线上故障。
- 但它已经影响项目事实表达，且会干扰后续开发、发布和排障判断，优先级应高于普通美化型优化。
- 如果近期还有功能开发或对外发布计划，这个需求应提前处理。
