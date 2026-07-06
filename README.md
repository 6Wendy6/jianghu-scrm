# Jianghu SCRM Business Test Baseline

这是线下门店私域 SCRM 的业务测试基线版本，目标是让业务、产品、测试和后续研发同学可以拉到本地，验证门店、导购、客户、标签、触达、客户群和交接等核心链路。

当前版本仍是 demo 到工程化基线的过渡态：已经具备 PostgreSQL、Redis、分页、缓存、异步任务、指标、压测脚本和基础安全开关，适合接入脱敏业务样本数据做功能验证和性能基线测试。

## 文档入口

- 产品说明：[docs/SCRM_PRODUCT_SPEC.md](./docs/SCRM_PRODUCT_SPEC.md)
- 业务验收清单：[docs/SCRM_ACCEPTANCE_CHECKLIST.md](./docs/SCRM_ACCEPTANCE_CHECKLIST.md)
- 稳定演示脚本：[docs/SCRM_DEMO_SCRIPT.md](./docs/SCRM_DEMO_SCRIPT.md)
- 测试数据安全：[docs/SCRM_TEST_DATA_SAFETY.md](./docs/SCRM_TEST_DATA_SAFETY.md)

## 快速开始

环境要求：

- Node.js 18+
- Go 1.22+
- Docker Desktop

拉取代码：

```bash
git clone git@github.com:6Wendy6/jianghu-scrm.git
cd jianghu-scrm
git checkout codex/business-test-baseline
```

安装前端依赖：

```bash
cd frontend
npm install
cd ..
```

启动 PostgreSQL 和 Redis：

```bash
npm run infra:up
```

启动后端有两种模式。

内存模式适合快速看原型，不需要数据库：

```bash
npm run dev:backend
```

PostgreSQL 模式适合真实开发，会读取 `.env` 中的 `DATABASE_URL` 或 `SCRM_DATABASE_URL`，并在启动时执行迁移：

```bash
npm run dev:backend:db
```

后端默认地址：

```text
http://127.0.0.1:8080
```

启动前端：

```bash
npm run dev
```

常用检查：

```bash
curl http://127.0.0.1:8080/api/health
curl http://127.0.0.1:8080/api/system/status
npm run test:backend
npm run check
npm run perf:smoke
```

## 本地数据库运行约定

本项目支持内存模式和 PostgreSQL 模式。为了避免误删本地数据，默认命令不会重置 Docker volume，也不会清空现有数据库。

推荐在 `.env` 中显式配置：

```bash
DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15432/scrm_dev?sslmode=disable
SCRM_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15432/scrm_dev?sslmode=disable
# DB 测试和 E2E 清理请使用临时/独立测试库，不要指向 15432：
# SCRM_TEST_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable
SCRM_AUTO_MIGRATE=true
```

如果你本机 `15432` 的旧数据卷密码不是 `scrm/scrm`，不要直接删除或重置 volume。更稳妥的做法是：

1. 保留旧数据卷。
2. 在 `.env` 写入真实可用的 `DATABASE_URL`。
3. 或单独新建 `scrm_dev` / `scrm_test` 标准库用于后续开发。

迁移、seed、状态检查：

```bash
npm run db:migrate
npm run db:seed
npm run db:status
```

`backend/migrations/` 只放 schema migration；本地演示数据放在 `backend/seeds/`。当前客户运营 demo seed 是：

```text
backend/seeds/customer_ops_demo_seed.sql
```

DB 测试需要 `SCRM_TEST_DATABASE_URL` 指向一个可写的临时/独立测试库；脚本会拒绝 `15432`，避免误碰本地开发 volume：

```bash
SCRM_TEST_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable npm run test:backend:db
```

如果没有测试库，普通测试仍可运行：

```bash
npm run test:backend
```

## 测试数据安全

E2E 测试、业务验收测试和接口冒烟测试必须隔离测试数据。优先使用临时 PostgreSQL 容器或 `SCRM_TEST_DATABASE_URL`，不要向真实 `DATABASE_URL` 写入测试数据。

核心规则：

- 不删除、重置或清空现有 `15432` PostgreSQL volume。
- 每轮测试生成唯一 `test_run_id`，例如 `e2e_20260706_153000`。
- 测试客户、门店、导购、标签、SOP、异常、企微配置都必须带 `test_run_id` 或 `[E2E_TEST_<test_run_id>]` 前缀。
- cleanup 必须先 dry-run，再 apply。
- 严禁无 `WHERE` 条件的 `DELETE`，严禁 `TRUNCATE` / `DROP` 清理业务表。

清理脚本：

```bash
SCRM_TEST_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable \
  npm run e2e:cleanup:dry-run -- --test-run-id=e2e_20260706_153000

SCRM_TEST_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable \
  npm run e2e:cleanup:apply -- --test-run-id=e2e_20260706_153000
```

完整规则见 [docs/SCRM_TEST_DATA_SAFETY.md](./docs/SCRM_TEST_DATA_SAFETY.md)。

## 业务数据测试建议

建议先使用脱敏样本数据，不要直接导入全量生产数据。

推荐顺序：

1. 准备 500 到 5000 条脱敏客户数据，包含客户、手机号掩码、企微昵称、来源门店、归属导购、标签、跟进记录和成交记录。
2. 准备门店和导购基础数据，保证客户的门店、导购关系能对上。
3. 先验证功能链路：客户查询、客户详情、导购关系、触达记录、打标签、异步批量打标签、交接。
4. 再做性能验证：从 `npm run perf:smoke` 开始，再逐步增加并发和目标 RPS。
5. 观察 `/api/metrics`：重点看请求延迟、DB 连接、Redis 命中率、任务积压和死信任务。

如果导入真实业务样本，优先新建独立测试库并在 `.env` 指向它，不要为了测试直接清空既有 Docker volume，避免误删本机已有数据。

## 已做的工程优化

- 运行保护：HTTP timeout、优雅停机、请求 ID、访问日志、CORS 头、每 IP token bucket 限流。
- 数据库基线：PostgreSQL 模式、连接池参数、自动迁移、幂等 seed、健康检查 DB pool stats。
- Redis 缓存：热读接口支持 Redis 分布式缓存，未配置 Redis 时回退本地短 TTL 缓存。
- 分页接口：客户、标签客户、任务、事件等列表支持分页，避免大列表一次性返回。
- 异步任务：批量打标签支持异步任务、任务项、worker 后台处理。
- worker 工业化：DB worker 支持 `FOR UPDATE SKIP LOCKED`、失败重试、重试延迟、死信状态和失败原因。
- 观测指标：`/api/metrics` 输出 Prometheus 风格指标，覆盖 HTTP、运行时、DB、缓存、事件/任务积压和死信任务。
- 性能脚本：`scripts/load-test.mjs` 支持本地压测，`npm run perf:smoke` 可快速验证热读性能。
- 业务时区：默认 `Asia/Shanghai`，Go 时间和 PostgreSQL 输出统一为业务时区。
- 基础安全：可选 `SCRM_API_TOKEN`，生产或共享测试环境可打开 API Token 保护。
- 结构拆分：后端已从单个大 `main.go` 拆成按职责划分的 Go 文件，方便后续多人协作。

## 业务模块说明

### 总览看板

接口集中在 `GET /api/summary` 和 `POST /api/metrics/refresh`。用于查看今日入池、归因率、待办、触达覆盖、趋势和运营建议。PostgreSQL 模式下读取 `metric_snapshots`，避免每次页面访问都实时聚合大表。

### 门店管理

覆盖门店基础信息、门店私域配置、门店导购、门店交接。核心接口包括：

- `GET /api/stores`
- `PATCH /api/stores/{id}/config`
- `GET/POST /api/stores/{id}/guides`
- `POST /api/stores/{id}/handover/sync`
- `POST /api/stores/{id}/handover/submit`

用于验证线下门店扫码入池、导购服务分配、离职/调店客户交接。

### 导购管理

覆盖导购列表、导购状态、导购活码生命周期和二维码下载。核心接口包括：

- `GET /api/guides`
- `POST /api/guides/{id}/pause`
- `POST /api/guides/{id}/remove`
- `GET /api/guides/{id}/lifecycle`
- `POST /api/guides/{id}/qr-download`

用于验证导购接待码、暂停接待、移除导购、生命周期审计。

### 客户管理

覆盖客户列表、客户详情、标签、导购关系、跟进、触达、线索、成交和流失。核心接口包括：

- `GET/POST /api/customers`
- `GET /api/customers/{id}`
- `POST /api/customers/{id}/touch`
- `POST /api/customers/{id}/followups`
- `POST /api/customers/{id}/relations`
- `POST /api/customers/{id}/lead`
- `POST /api/customers/{id}/order`
- `POST /api/customers/{id}/lost`

用于验证客户从入池、运营、转化到复购/流失的闭环。

### 标签与规则

覆盖标签字典、标签分组、客户标签查询、自动标签规则和预打标签规则。核心接口包括：

- `GET/POST /api/tags`
- `PATCH /api/tags/{name}/toggle`
- `PATCH /api/tags/{name}/rename`
- `GET /api/tags/{name}/customers`
- `GET/POST /api/tag-groups`
- `GET/POST /api/tag-rules/auto`
- `GET/POST /api/tag-rules/pre`

用于验证客户分层、来源识别、活动预标签和后续触达筛选。

### 触达运营

覆盖欢迎语、群发任务、SOP 等触达配置。核心接口：

- `GET/POST /api/touches`

用于验证从标签或客户状态出发创建触达规则。

### 客户群运营

覆盖客户群、群发、入群欢迎语、群 SOP、群日历、群提醒和客户群标签。核心接口包括：

- `GET/POST /api/groups`
- `GET/POST /api/groups/mass`
- `GET/POST /api/groups/welcomes`
- `GET/POST /api/groups/sops`
- `GET/POST /api/groups/calendar`
- `GET/POST /api/groups/reminders`
- `GET/POST /api/groups/tag-groups`

用于验证门店社群运营、活动群运营和群内提醒。

### 事件与异步任务

事件收件箱用于未来接企微回调、扫码事件、客户变更事件。异步任务用于高批量操作，例如批量打标签。

- `GET/POST /api/events/inbox`
- `GET/POST /api/tasks`
- `GET /api/tasks/{id}`
- `GET /api/tasks/{id}/items`
- `POST /api/customers/batch-tags`
- `POST /api/customers/batch-tags-async`

大批量业务测试优先使用异步接口，避免把请求线程长期占住。

### 企业微信集成中心

当前阶段只做真实企业微信接入前的底座检查，不做完整客户同步。核心目标是让配置、token、权限、活码更新、回调和错误原因可检查、可解释、可恢复。

- `GET /api/scrm/wecom/config`: 读取企业微信接入配置，返回时会隐藏 secret、回调 token 和 AESKey。
- `PUT /api/scrm/wecom/config`: 保存 `corpId`、通讯录 secret、客户联系 secret、应用 agentId/secret、回调 token、EncodingAESKey 和启用状态；secret 留空会保留已有密文。
- `POST /api/scrm/wecom/test-token`: 测试通讯录、客户联系或应用 token，缓存 token 但响应不返回 access_token 明文。
- `POST /api/scrm/wecom/permission-check`: 检测 token、部门读取、成员读取和客户联系基础权限；失败项会写入异常监控。
- `GET /api/scrm/wecom/permission-checks`: 查看最近权限检测历史。
- `GET /api/scrm/wecom/status`: 企业微信集成中心状态，包含配置脱敏信息、token 缓存状态、权限检测摘要、资产数量、重试任务、最近回调和下一步建议。
- `GET /api/scrm/wecom/error-dictionary`: 常见企业微信错误码解释和处理建议。
- `GET /api/scrm/wecom/doctor`: 企微接入 doctor 检查。
- `GET /api/scrm/wecom/retries`: 查看 `update_contact_way` 失败重试任务。
- `POST /api/scrm/wecom/retries/{id}/retry`: 手动重试失败任务。
- `GET /api/scrm/wecom/integration-log`: 查看最近回调和重试日志。

`/api/system/status` 也会返回 `wecom` 摘要，方便演示和排障时快速判断当前接入底座状态。

## 关键环境变量

参考 [.env.example](./.env.example)。

常用变量：

- `DATABASE_URL`: PostgreSQL 连接串别名；`SCRM_DATABASE_URL` 未设置时会使用它。
- `SCRM_DATABASE_URL`: PostgreSQL 连接串。
- `SCRM_TEST_DATABASE_URL`: 真实 PostgreSQL 行为测试使用的测试库连接串。
- `SCRM_DATABASE_MODE`: 文档化模式标识，建议 `memory` 或 `postgres`。
- `BACKEND_PORT`: 未设置 `SCRM_API_ADDR` 时用于推导后端端口。
- `FRONTEND_PORT`: 前端开发服务端口建议值。
- `SCRM_REDIS_ADDR`: Redis 地址。
- `SCRM_AUTO_MIGRATE`: 是否启动时执行迁移。
- `SCRM_API_TOKEN`: 可选 API Token。
- `SCRM_BUSINESS_TIME_ZONE`: 业务时区，默认 `Asia/Shanghai`。
- `SCRM_DB_MAX_OPEN_CONNS`: 单实例最大 DB 连接数。
- `SCRM_WORKER_BATCH_SIZE`: worker 每轮处理数量。
- `SCRM_WORKER_MAX_ATTEMPTS`: worker 最大重试次数。
- `SCRM_WORKER_RETRY_DELAY`: worker 失败后的重试间隔。

## 压测入口

快速冒烟：

```bash
npm run perf:smoke
```

自定义压测：

```bash
DURATION_SECONDS=60 CONCURRENCY=100 TARGET_RPS=300 npm run perf:load
```

2,000 到 3,000 在线用户不等于 2,000 到 3,000 并发请求。建议先从 100 到 300 并发 worker、200 到 500 RPS 的热读/混合读写压测开始，观察 P95、DB wait count、Redis 命中率、任务积压和死信数。

## 当前边界

- 真实企微回调验签、加解密、消息去重和外部接口限流还需要继续接入。
- 多租户、组织、角色权限和门店级数据隔离还未完全落地。
- 企业微信集成中心当前是接入底座，不代表已经完成部门、成员、客户联系、客户群的完整真实同步。

## 客户运营中心持久化说明

客户运营中心的 PostgreSQL schema 在 `backend/migrations/011_customer_ops_persistence.sql`，演示数据在 `backend/seeds/customer_ops_demo_seed.sql`。

DB 模式下，生命周期、标签、跟进、SOP 任务、素材和异常监控走 repository/service 分层，并在关键动作中使用事务写入主表、日志表和客户时间线。前端原型右下角会显示当前后端模式、数据库连接和迁移版本，便于区分当前跑的是 memory 还是 postgres。
- 业务数据导入脚本还未内置，当前建议先按表结构或接口导入脱敏样本。
- 目前是业务测试基线，不是最终生产发布包。
