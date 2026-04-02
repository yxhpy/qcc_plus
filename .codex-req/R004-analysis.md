# R004 需求分析：重新部署流程脚本化并强制统一入口

## 1. 需求定性

**结论**：`优化`

**判断依据**：
- 仓库内已经存在 `scripts/deploy-server.sh`，且该脚本已覆盖“拉取最新代码、构建、重启容器、健康检查”等核心步骤，不属于从零新增部署能力。
- 当前问题的核心是**部署入口不一致**：有的路径调用脚本，有的路径在脚本前后又执行了额外命令，文档里也保留了直接 `git pull && docker compose up -d --build` 的非脚本路径。
- 该需求更偏向**流程收敛、运维一致性和防漂移治理**，目标是让测试/生产重新部署都固定走单一脚本入口，减少环境差异和人为遗漏。

补充判断：
- 这不是典型功能型新需求。
- 这也不完全是线上故障型 Bug，因为核心脚本能力已存在，问题在于执行口径不统一，属于高价值流程优化。

## 2. 涉及文件列表（预估）

以下为**高概率涉及**文件：

- `.github/workflows/deploy-test.yml`
- `.github/workflows/deploy-prod.yml`
- `scripts/deploy-server.sh`
- `docs/ci-cd-deployment.md`
- `docs/claude/deployment-private.md`
- `docs/release-workflow.md`
- `README.md`

以下为**需要核对但未必修改**的文件：

- `scripts/deploy-test-local.sh`
- `scripts/start_proxy_docker.sh`
- `docs/modules/REGISTRY.md`
- `docs/claude/lessons-learned.md`

说明：
- 当前仓库已经有统一部署脚本，预计改动重点会集中在 **CI 工作流、手动部署文档、脚本边界定义**。
- 后端 Go 代码和前端 React 代码大概率**不需要直接改动**。

## 3. 依赖检查

### 3.1 现有能力检查

- 已有统一部署脚本：`scripts/deploy-server.sh`
- 已有环境区分：`test` / `prod`
- 已有构建流程：脚本内执行 `npm ci` 与 `bash scripts/build-frontend.sh`
- 已有容器重建流程：脚本内执行 `docker compose down` 和 `docker compose up -d --build --force-recreate`
- 已有健康检查：脚本内使用 `curl` 对 `127.0.0.1:<port>/` 做重试探活
- 已有 CI 入口：`.github/workflows/deploy-test.yml`、`.github/workflows/deploy-prod.yml`

### 3.2 已发现的不一致点

- `.github/workflows/deploy-test.yml` 在调用 `scripts/deploy-server.sh` 之前，又额外执行了一次 `git reset/git clean/git fetch/git checkout/git pull`，与“统一由脚本负责部署步骤”的目标不一致。
- `.github/workflows/deploy-prod.yml` 直接调用脚本，和测试环境流程口径不一致。
- `docs/claude/deployment-private.md` 仍保留 `git pull && docker compose ... up -d --build` 的手工部署方式，绕过了统一脚本。
- `docs/release-workflow.md` 仍有 `docker compose pull`、`docker compose up -d` 的直接部署描述，容易继续扩散非脚本入口。
- `README.md` 的 `docker compose up -d` 更偏本地快速启动，不一定属于服务器重新部署，但需要明确边界，避免与“正式部署流程”混淆。

### 3.3 外部依赖检查

本需求预计**不需要新增 Go / npm 依赖**，主要依赖仍为：

- `git`
- `docker`
- `docker compose` 或 `docker-compose`
- `curl`
- `node` / `npm`（因为脚本内会构建前端）
- GitHub Actions 的 `appleboy/ssh-action`

需要确认的运行前提：
- 目标服务器具备 Node.js / npm，否则现有 `deploy-server.sh` 的前端构建步骤会失败。
- 目标服务器具备 `curl`，否则健康检查无法执行。
- `.env`、Compose 文件与固定容器命名规则已经在目标环境稳定可用。

## 4. 实现方案概述（3-5 步）

1. 以 `scripts/deploy-server.sh` 作为测试/生产环境的唯一重新部署入口，明确脚本负责拉代码、构建、重启容器、健康检查全流程。
2. 精简 `.github/workflows/deploy-test.yml` 和 `.github/workflows/deploy-prod.yml`，移除脚本外重复的拉代码或部署命令，只保留 SSH 到目标机后执行 `./scripts/deploy-server.sh <env>`。
3. 清理文档中的非脚本式正式部署命令，把服务器部署、手动重部署、故障恢复等场景统一改为脚本调用口径；本地快速启动文档则明确标注“仅本地开发/体验，不等同正式部署”。
4. 视需要补强 `scripts/deploy-server.sh` 的参数说明、日志提示和失败信息，让脚本成为唯一入口后更易排障。
5. 回归验证测试/生产两条链路，确认无论是 GitHub Actions 触发，还是服务器人工重部署，最终都走同一脚本且步骤一致。

## 5. 风险点

- **脚本强制化后的误删风险**：`scripts/deploy-server.sh` 当前会执行 `git reset --hard HEAD` 和 `git clean -fd`。如果服务器上存在未提交但又不该被清理的本地文件，强制走脚本会把这类隐式状态暴露出来。
- **环境前置依赖风险**：脚本内包含 `npm ci` 和前端构建，若服务器 Node/npm 版本不符合要求，统一脚本后会让所有部署都卡在同一前置依赖上。
- **文档边界风险**：本地开发的 `docker compose up -d` 不应被误删为“违规路径”；需要明确区分“本地启动”与“服务器正式重部署”。
- **重复健康检查风险**：当前脚本内有一次健康检查，Actions 外层还有一次对外探活。双重检查是合理的，但文档要说明职责边界，避免后续再次在工作流里堆叠重复逻辑。
- **历史运维习惯切换风险**：如果维护者已经习惯直接 `docker compose up -d --build`，统一脚本后需要同步修正文档和操作手册，否则仍会回到老路径。

## 6. 建议：接受/拒绝（附原因）

**建议：接受**

**原因**：
- 该需求投入小、收益明确，主要是收敛流程入口，不涉及大规模业务代码改造。
- 当前仓库已经具备可复用的部署脚本基础，实现成本较低，属于“已有能力未彻底落地”的问题。
- 统一入口后，测试环境和生产环境的行为更可预测，排障会更直接，文档和 CI 也更容易维护。
- 这类问题如果不处理，后续会持续出现“同样叫部署，但不同人执行的步骤不同”的隐性故障和文档漂移。

补充建议：
- 接受后应明确范围是“**正式服务器重部署流程统一走脚本**”，不要误伤本地开发/快速体验命令。
- 如果要进一步提高约束，可以考虑把脚本外的手工命令从文档里完全移除，只保留脚本入口。

## 7. 预估优先级（1-5，1 最高）

**优先级：2**

说明：
- 这不是直接导致功能不可用的业务 Bug，因此通常不排到 1。
- 但它直接影响部署稳定性、环境一致性和后续故障定位效率，优先级明显高于一般文档优化。
- 如果近期仍有频繁发布或测试/生产口径差异问题，实际上可以按接近 P1 的方式优先处理。
