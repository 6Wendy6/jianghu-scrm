# Jianghu SCRM Business Test Baseline

这是线下门店私域 SCRM 的业务测试基线版本，目标是让业务、产品、测试和后续研发同学可以拉到本地，验证门店、导购、客户、标签、触达、客户群和交接等核心链路。

当前版本仍是 demo 到工程化基线的过渡态：已经具备 PostgreSQL、Redis、分页、缓存、异步任务、指标、压测脚本和基础安全开关，适合接入脱敏业务样本数据做功能验证和性能基线测试。

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

启动后端，自动执行迁移并使用 Redis 缓存：

```bash
npm run dev:backend:db:redis
```

后端默认地址：

```text
http://127.0.0.1:8080
```

启动前端：

```bash
npm run dev:frontend
```

常用检查：

```bash
curl http://127.0.0.1:8080/api/health
npm run check
npm run perf:smoke
```

## 业务数据测试建议

建议先使用脱敏样本数据，不要直接导入全量生产数据。

推荐顺序：

1. 准备 500 到 5000 条脱敏客户数据，包含客户、手机号掩码、企微昵称、来源门店、归属导购、标签、跟进记录和成交记录。
2. 准备门店和导购基础数据，保证客户的门店、导购关系能对上。
3. 先验证功能链路：客户查询、客户详情、导购关系、触达记录、打标签、异步批量打标签、交接。
4. 再做性能验证：从 `npm run perf:smoke` 开始，再逐步增加并发和目标 RPS。
5. 观察 `/api/metrics`：重点看请求延迟、DB 连接、Redis 命中率、任务积压和死信任务。

如果导入真实业务样本，优先新建测试库或清空本地 Docker volume，避免 demo seed 数据和业务测试数据混在一起。

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

## 关键环境变量

参考 [.env.example](./.env.example)。

常用变量：

- `SCRM_DATABASE_URL`: PostgreSQL 连接串。
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
- 业务数据导入脚本还未内置，当前建议先按表结构或接口导入脱敏样本。
- 目前是业务测试基线，不是最终生产发布包。
