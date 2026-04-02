# R002 需求分析：解决 Docker 重部署后的数据与配置持久化问题

## 1. 需求定性

**结论：Bug**

**原因：**
- 需求描述是“每次重新部署 qcc_plus 数据都丢失”，这是现有行为与预期不符，不是新增能力。
- 当前项目口径已经明确支持持久化：
  - 默认使用 SQLite，本地路径默认 `~/.qccplus/qccplus.db`
  - 设置 `PROXY_MYSQL_DSN` 后切换到 MySQL
  - `docker-compose*.yml` 已为 MySQL 配置数据卷
- 因此本需求本质上是在修复“Docker 部署链路没有把持久化闭环做好”的问题。

**补充判断：**
- 主体是部署层 Bug。
- 同时带有少量运维优化属性，因为需要顺带收口部署脚本、Compose 模板和文档口径。

## 2. 涉及文件列表（预估）

### 高概率直接改动

- `docker-compose.yml`
- `docker-compose.test.yml`
- `docker-compose.prod.yml`
- `.env.example`
- `scripts/start_proxy_docker.sh`
- `scripts/deploy-server.sh`
- `scripts/diagnose-data.sh`
- `README.md`
- `README.dockerhub.md`
- `docs/ci-cd-deployment.md`
- `docs/docker-cli-health-check-deployment.md`
- `docs/cloudflare-tunnel.md`

### 需要作为事实源核对

- `cmd/cccli/main.go`
- `internal/proxy/builder.go`
- `internal/store/store.go`
- `internal/store/sqlite.go`
- `Dockerfile`
- `.gitignore`

### 可能视方案补充改动

- `docs/README.md`
- `docs/claude/lessons-learned.md`

## 3. 依赖检查

### 3.1 现有能力检查

- **MySQL 持久化能力已存在**
  - `cmd/cccli/main.go` 读取 `PROXY_MYSQL_DSN`
  - `internal/store/store.go` 已支持 MySQL 连接、迁移和连接池
- **SQLite 持久化能力已存在**
  - `internal/proxy/builder.go` 默认会落到 `~/.qccplus/qccplus.db`
  - `internal/store/sqlite.go` 会自动创建目录
- **Compose 已有 MySQL 数据卷**
  - `docker-compose.yml` 使用 `mysql_data`
  - `docker-compose.test.yml` 使用 `mysql_data_test`
  - `docker-compose.prod.yml` 使用 `mysql_data_prod`
- **CI/CD 部署脚本默认不会删卷**
  - `scripts/deploy-server.sh` 使用 `docker compose down --remove-orphans`
  - 当前没有 `-v`，理论上不会主动删除 named volume

### 3.2 已确认的问题

- **配置来源不统一**
  - `docs/ci-cd-deployment.md` 要求服务器维护 `.env`
  - 但 `docker-compose.test.yml` / `docker-compose.prod.yml` 中大部分关键变量仍是硬编码，不是真正从 `.env` 注入
  - 结果是：用户以为配置保存在 `.env`，实际重部署时仍可能回到 compose 文件里的默认值

- **普通 Compose 路径与 CI/CD 路径的持久化口径不一致**
  - CI/CD 使用固定 `-p qcc_test` / `-p qcc_prod`，卷复用更稳定
  - README 和其他文档大量使用普通 `docker compose up -d`
  - 普通方式依赖当前目录名决定 project name，换目录、换仓库路径、换项目名时，Docker 会创建新卷，表现为“数据丢失”

- **单容器部署示例没有持久化设计**
  - `README.dockerhub.md` / 相关 Docker 文档中的 `docker run` 示例没有挂载数据卷或配置目录
  - 如果用户走单容器部署，容器重建后数据天然会丢

- **代理容器本地状态未形成清晰持久化约定**
  - 当前主要持久化只覆盖 MySQL
  - 若用户未正确启用 MySQL，系统会回退 SQLite 到 `~/.qccplus/qccplus.db`
  - Compose 没有为 `proxy` 容器显式挂载对应目录，SQLite 模式下重建容器仍会丢数据

### 3.3 依赖结论

- **不需要新增 Go / npm 依赖**
- 主要依赖仍是：
  - Docker / Docker Compose
  - MySQL 8.0
  - 现有环境变量机制
- 改动重点应放在：
  - Compose 模板
  - 部署脚本
  - 文档与验证脚本

## 4. 实现方案概述（3-5 步）

1. **统一配置注入方式**
   - 让 `docker-compose.yml`、`docker-compose.test.yml`、`docker-compose.prod.yml` 通过 `env_file` 或 `${VAR}` 插值真正消费 `.env`
   - 把 MySQL 账号、密码、DSN、端口、管理员密钥等关键配置从硬编码迁移到 `.env`

2. **固定持久化路径或卷名**
   - MySQL 继续持久化到稳定卷，但要避免依赖目录名隐式生成卷名
   - 可选方案：
     - 使用显式命名卷 `name: ...`
     - 或改成宿主机 bind mount，例如 `./.docker/data/mysql`
   - 目标是“换目录重部署、脚本重部署、手动重部署”都落到同一份数据

3. **补齐 proxy 本地状态持久化**
   - 为 `proxy` 服务增加稳定挂载目录
   - 显式约定 `PROXY_SQLITE_PATH` 指向持久化目录中的 SQLite 文件
   - 这样即使用户未启用 MySQL，也不会因容器重建而丢失本地数据和配置

4. **校准部署脚本与文档**
   - `scripts/start_proxy_docker.sh`、`scripts/deploy-server.sh` 明确使用统一 project name、统一 `.env`
   - 文档明确说明：
     - 配置保存在 `.env`
     - 数据保存在 volume 或宿主机目录
     - 不要使用 `docker compose down -v`
     - 不要把线上配置直接改在 compose 文件里

5. **增加可复现的回归验证**
   - 验证流程至少覆盖：
     - 首次部署
     - 写入 MySQL 数据
     - 修改管理配置
     - 执行重部署
     - 校验数据与配置仍存在
   - 必要时扩展 `scripts/diagnose-data.sh`

## 5. 风险点

- **卷迁移风险**
  - 如果从当前默认 named volume 改成显式卷名或 bind mount，旧环境需要迁移说明
  - 否则会出现“新方案已生效，但历史数据还在旧卷里”的假性丢失

- **配置边界不清风险**
  - 这里的“配置”至少可能包含三类：
    - `.env` 中的部署配置
    - SQLite / MySQL 中的系统配置
    - 容器内 `~/.qccplus` 本地文件
  - 实施前需要明确本需求要求全部保留，还是优先保留部署配置与数据库数据

- **兼容多部署方式风险**
  - 不能只修 CI/CD 路径
  - 还要覆盖 README 里的 `docker compose up -d` 和 `docker run` 场景，否则用户仍会继续遇到同类问题

- **错误使用 Docker 命令的风险**
  - 即使代码修好，用户若手动执行 `docker compose down -v` 或 `docker volume rm`，数据仍会被删
  - 文档需要明确禁止项和恢复建议

- **安全风险**
  - 配置改成 `.env` 驱动后，需要继续确保 `.env` 不入库、不进镜像、不出现在公开示例里

## 6. 建议：接受 / 拒绝（附原因）

**建议：接受**

**原因：**
- 这是高严重度问题，直接影响升级、回滚、重建和迁移的可靠性。
- 当前仓库已经具备大部分持久化能力，说明问题主要集中在部署模板和配置管理，修复成本可控。
- 该问题具有重复触发特征，不修的话每次部署都可能继续踩坑。
- 修复后可以统一 Docker、CI/CD 和 README 的部署口径，降低后续维护成本。

**补充说明：**
- 从当前代码事实看，“MySQL 本身不支持持久化”并不成立，问题更偏向部署实现不稳定。
- 因此这是值得处理且收益很高的修复项。

## 7. 预估优先级（1-5，1 最高）

**优先级：1**

**原因：**
- 数据与配置丢失属于核心运维故障，不是普通体验问题。
- 影响面覆盖测试、生产、手动部署、CI/CD 重部署等多个关键路径。
- 若继续带病发布，会直接影响用户对项目可用性和升级安全性的判断。
