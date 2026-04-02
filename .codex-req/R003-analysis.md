# R003 需求分析：开发完成后仅自动部署测试环境，正式环境改为人工触发，开发流程结束后不自动发布

## 1. 需求定性

**结论**：优化类需求（偏 CI/CD 流程治理与发布门禁加强）。

**判断依据**：
- 需求目标不是新增业务功能，也不是修复运行时缺陷，而是调整现有研发交付流程的触发条件。
- 当前仓库已经具备测试环境自动部署、正式环境自动部署、Tag 自动发布能力；本需求是把“自动执行”收紧为“测试自动、正式手动、发布不跟随开发流程自动发生”。
- 该需求本质上属于**发布安全性优化**和**流程风险控制**，重点在于降低误推正式、误发布的概率。

补充说明：
- 当前实现里，`test` 分支推送会自动部署测试环境，这与需求一致。
- 当前实现里，`prod` 分支推送会自动部署正式环境，这与需求冲突。
- 当前实现里，`release.yml` 仅在推送 `v*.*.*` tag 时触发，严格说并不是“开发结束自动发布”；但如果项目希望“正式发布也必须显式审批/手动触发”，则该工作流也需要纳入调整范围。

## 2. 涉及文件列表（预估）

以下为**高概率直接涉及**文件：

- `.github/workflows/deploy-test.yml`
- `.github/workflows/deploy-prod.yml`
- `.github/workflows/release.yml`
- `scripts/deploy-server.sh`
- `docs/ci-cd-deployment.md`
- `docs/release-workflow.md`
- `docs/claude/git-workflow.md`
- `docs/claude/release-policy.md`
- `docs/claude/deployment-private.md`

以下为**需要作为事实源核对**的文件：

- `docker-compose.test.yml`
- `docker-compose.prod.yml`
- `docker-compose.yml`
- `README.md`
- `AGENTS.md`

补充说明：
- 需求描述写的是“正式环境（`docker-compose.yml:8000`）”，但当前正式部署脚本实际使用的是 `docker-compose.prod.yml`，端口也是 `8000`。
- `docker-compose.yml` 当前更像默认/本地 compose 入口，而不是现有 GitHub Actions 正式部署入口；这个口径差异必须在实施前先统一。

## 3. 依赖检查

### 3.1 当前自动化链路现状

- `.github/workflows/deploy-test.yml`
  - 触发条件：`push` 到 `test`
  - 行为：自动 SSH 到服务器，执行 `./scripts/deploy-server.sh test`
  - 部署目标：测试环境，端口 `8001`，compose 文件为 `docker-compose.test.yml`

- `.github/workflows/deploy-prod.yml`
  - 触发条件：`push` 到 `prod`
  - 行为：自动 SSH 到服务器，执行 `./scripts/deploy-server.sh prod`
  - 部署目标：正式环境，端口 `8000`，compose 文件为 `docker-compose.prod.yml`

- `.github/workflows/release.yml`
  - 触发条件：`push` `v*.*.*` tag
  - 行为：GoReleaser 发布 GitHub Release 和 Docker Hub 镜像
  - 结论：当前“发布”已是显式 tag 触发，不跟随普通开发推送自动发生，但一旦 tag 被推送，仍会自动执行整套发布动作

### 3.2 与现有需求记录的交叉

- 与 `R002` 有明显文件交叉：
  - `scripts/deploy-server.sh`
  - `docs/ci-cd-deployment.md`
  - `docker-compose.test.yml`
  - `docker-compose.prod.yml`
- 原因：
  - `R002` 关注重部署后的数据/配置持久化；
  - 本需求关注部署触发策略；
  - 两者都落在同一套 CI/CD 与 compose 体系里，实施时需要避免互相覆盖。

- 与 `R001` 有文档交叉：
  - `README.md`
  - `docs/release-workflow.md`
  - `docs/claude/git-workflow.md`
  - `docs/claude/release-policy.md`
- 原因：
  - 当前文档已把 “push prod -> 自动部署正式” 写成流程规则；
  - 本需求会直接改变这些文档的事实口径。

### 3.3 技术与平台依赖

- 预计**不需要新增 Go 依赖、npm 依赖或数据库依赖**。
- 主要依赖仍是：
  - GitHub Actions 事件模型（`push` / `workflow_dispatch` / 可能的 environment approval）
  - 现有 SSH Secrets
  - 现有部署脚本 `scripts/deploy-server.sh`
  - 现有服务器目录与 compose 文件

### 3.4 需求澄清点

以下两点在实施前需要明确：

- “用户主动发起推正式请求”具体指什么：
  - GitHub Actions 手动点按钮 `workflow_dispatch`
  - 推送 `prod` 分支
  - 打正式 tag
  - 通过 issue/comment/slash command 触发

- “开发流程结束后不自动发布”具体约束到哪一层：
  - 仅指**不自动部署正式环境**
  - 还是也包括**不自动推 Docker Hub / 不自动创建 GitHub Release**

从当前仓库复杂度和可维护性看，**优先建议使用 `workflow_dispatch` 作为正式部署触发方式**，比评论命令或自定义 webhook 更简单、更稳。

## 4. 实现方案概述（3-5 步）

1. 先统一流程口径。
   明确“自动部署”只保留 `test` 分支，正式环境改成显式人工触发；同时确认正式环境到底使用 `docker-compose.prod.yml` 还是要切换为 `docker-compose.yml`。

2. 调整正式部署工作流。
   将 `.github/workflows/deploy-prod.yml` 从 `push: branches: [prod]` 改为人工触发模式（优先 `workflow_dispatch`），必要时增加 `ref/branch` 输入参数，并保留现有 SSH 部署与健康检查逻辑。

3. 校准部署脚本与分支约束。
   评估 `scripts/deploy-server.sh prod` 是否仍固定拉取 `prod` 分支，还是改成允许显式传入待部署 ref；避免“人工点了正式部署，但实际部署内容仍不透明”。

4. 收紧发布链路。
   如果目标是“只有用户明确发起正式请求时，才允许发布 Docker Hub / GitHub Release”，则同步调整 `.github/workflows/release.yml`，改为人工触发或审批式 tag 流程；如果只要求“不自动上正式环境”，则保留 tag 发布模式并在文档中写清楚。

5. 更新文档与操作手册。
   同步修正文档中关于 `test / main / prod / tag` 的职责、正式部署入口、发布时间点、人工触发步骤和回滚方式，避免流程代码改了但文档仍指导用户 `git push origin prod`。

## 5. 风险点

- **流程口径不一致风险**：需求写的是 `docker-compose.yml:8000`，当前实现正式部署实际用的是 `docker-compose.prod.yml:8000`；如果不先统一，后续容易改错文件。
- **人工触发语义不清风险**：如果“推正式请求”没有固定入口，最终可能出现多种绕行方式，导致流程又重新失控。
- **分支与环境漂移风险**：取消 `push prod` 自动部署后，`prod` 分支状态与线上正式环境可能不再天然一致，需要记录“当前生产部署的是哪个 commit/tag”。
- **发布与部署边界混淆风险**：如果只改 `deploy-prod.yml`，但 `release.yml` 仍保持 tag 自动发布，可能会出现“没有自动部署正式，但已经自动把正式镜像推到 Docker Hub”的半收紧状态。
- **文档滞后风险**：当前多份文档都把现有流程写死，一旦代码先改、文档不跟，团队成员仍会按旧方式操作。
- **运维效率下降风险**：加人工门禁会降低误发布，但也会增加一步操作；如果没有设计成足够明确的手动入口，后续上线效率会变差。

## 6. 建议：接受/拒绝（附原因）

**建议：接受**

**原因**：
- 该需求直接降低“误推正式即上线”的风险，属于高价值流程治理项。
- 当前正式环境自动部署依赖 `push prod`，门槛过低，不利于正式环境变更控制。
- 现有测试环境自动部署已经满足快速迭代诉求，没有必要把同样的自动化强度延续到正式环境。
- 该改动主要集中在 GitHub Actions、部署脚本和文档，不需要引入新的运行时技术栈，实施成本可控。

**接受前建议增加一条前置确认**：
- 正式请求的唯一入口是否定为 GitHub Actions 的手动触发。
- 正式发布是否也必须人工触发，还是仅正式部署必须人工触发。

如果这两点不先定清楚，实施后仍可能出现“正式部署被收口了，但正式镜像依旧自动发布”或“有多个正式入口并存”的问题。

## 7. 预估优先级（1-5，1 最高）

**优先级：2**

说明：
- 这不是直接影响线上功能正确性的 Bug，因此不应高于持久化、数据安全、服务不可用等 P1 问题。
- 但它直接关系到正式环境变更控制，能显著降低误上线和误发布风险，优先级应高于一般体验型优化。
- 如果近期仍有频繁上线或多人协作，建议尽快处理。
