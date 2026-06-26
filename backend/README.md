# AI SCRM Go Backend

This backend is the first API implementation for the offline-store private-domain MVP prototype. It uses in-memory state so frontend modules can connect to real business actions before database modeling is finalized.

## Run

```bash
go run .
```

Default address:

```text
http://127.0.0.1:8080
```

Use another port when 8080 is occupied:

```bash
SCRM_API_ADDR=127.0.0.1:18080 go run .
```

## Runtime Guardrails

The API process is intended to stay stateless so it can run behind a load balancer when online usage grows. The current demo backend already enables the following production-facing guardrails:

- HTTP server timeouts for slow-client protection.
- Graceful shutdown on `SIGINT` / `SIGTERM`.
- Per-request `X-Request-ID` response header.
- Access logs with request id, method, path, status, duration, bytes, and remote address.
- Basic per-client-IP token-bucket rate limiting.
- Optional API token protection for production and shared environments.
- Configurable business timezone for API timestamps and PostgreSQL sessions.
- Short-TTL response cache for hot read paths in PostgreSQL mode.

## Source Layout

The backend keeps one Go package for now, but code is split by responsibility so new production capabilities do not keep growing `main.go`:

- `main.go`: process entry, server wiring, graceful shutdown.
- `config_db.go`: runtime config, PostgreSQL connection, migrations.
- `middleware_cache_metrics.go`: request middleware, rate limiting, Redis/local cache, Prometheus metric primitives.
- `models.go`: shared API/domain models.
- `api_setup.go`: demo bootstrap data, route registration, JSON error wrapper, health endpoint.
- `dashboard.go`: summary and metric snapshot queries.
- `stores.go`, `guides.go`, `handover.go`: store, guide, and handover workflows.
- `customers.go`, `customer_actions.go`, `batch_tags.go`: customer profile, operations, and batch tagging.
- `tags.go`, `groups_rules.go`: tag governance, customer groups, touch rules, and tag rules.
- `events_tasks.go`, `workers.go`: event inbox, async tasks, and background workers.
- `utils.go`: shared helpers.

Configurable environment variables:

| Variable | Default | Purpose |
|---|---:|---|
| `SCRM_API_ADDR` | `127.0.0.1:8080` | API listen address |
| `SCRM_READ_HEADER_TIMEOUT` | `2s` | Maximum time to read request headers |
| `SCRM_READ_TIMEOUT` | `10s` | Maximum time to read a full request |
| `SCRM_WRITE_TIMEOUT` | `15s` | Maximum time to write a response |
| `SCRM_IDLE_TIMEOUT` | `60s` | Keep-alive idle timeout |
| `SCRM_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown budget |
| `SCRM_RATE_LIMIT_RPS` | `80` | Allowed requests per second per client IP; set `0` to disable |
| `SCRM_RATE_LIMIT_BURST` | `160` | Burst capacity per client IP |
| `SCRM_API_TOKEN` | empty | Optional shared API token. When set, non-health API requests must send `Authorization: Bearer <token>` or `X-SCRM-API-Token` |
| `SCRM_BUSINESS_TIME_ZONE` | `Asia/Shanghai` | Business timezone used by Go timestamps and PostgreSQL sessions |
| `SCRM_WORKER_INTERVAL` | `500ms` | Background worker polling interval |
| `SCRM_WORKER_BATCH_SIZE` | `20` | Maximum queued events/task items processed per tick |
| `SCRM_WORKER_MAX_ATTEMPTS` | `3` | Maximum worker attempts before a task item enters `dead_letter` |
| `SCRM_WORKER_RETRY_DELAY` | `2s` | Delay before retrying a failed queued task item |
| `SCRM_DATABASE_URL` | empty | PostgreSQL DSN. Empty keeps the demo in memory mode |
| `SCRM_DB_MAX_OPEN_CONNS` | `25` | PostgreSQL max open connections per API instance |
| `SCRM_DB_MAX_IDLE_CONNS` | `10` | PostgreSQL max idle connections per API instance |
| `SCRM_DB_CONN_MAX_LIFETIME` | `30m` | Maximum PostgreSQL connection lifetime |
| `SCRM_DB_PING_TIMEOUT` | `2s` | Startup PostgreSQL ping timeout |
| `SCRM_AUTO_MIGRATE` | `false` | Apply `migrations/*.sql` on startup |
| `SCRM_CACHE_TTL` | `5s` | Response cache TTL; set `0` to disable |
| `SCRM_CACHE_MAX_ENTRIES` | `512` | Maximum local in-process response cache entries |
| `SCRM_REDIS_ADDR` | empty | Optional Redis address for distributed response cache |
| `SCRM_REDIS_PASSWORD` | empty | Optional Redis password |
| `SCRM_REDIS_DB` | `0` | Redis logical database |
| `SCRM_REDIS_KEY_PREFIX` | `scrm:cache:` | Redis cache key prefix |

For local stress checks:

```bash
SCRM_API_ADDR=127.0.0.1:18080 SCRM_RATE_LIMIT_RPS=20 SCRM_RATE_LIMIT_BURST=40 go run .
```

## PostgreSQL Mode

The backend now supports two storage modes:

- `memory`: default mode for the current clickable demo.
- `postgres`: enabled when `SCRM_DATABASE_URL` is provided.

The first PostgreSQL step is intentionally conservative: the service opens a tuned connection pool, can apply migrations, reports DB health, and keeps the existing in-memory API behavior until repositories are migrated endpoint by endpoint.

Migrated repository paths:

- `GET /api/stores`, `PATCH/POST /api/stores/{id}/config`: DB-backed in PostgreSQL mode for store profiles and private-domain config.
- `GET/POST /api/stores/{id}/guides`: DB-backed in PostgreSQL mode for guide QR lifecycle.
- `POST /api/stores/{id}/handover/sync`, `POST/PATCH /api/stores/{id}/handover/submit`: DB-backed in PostgreSQL mode for guide leave/transfer handover; submit updates customer owner, guide relations, handover status, and customer events in one transaction.
- `GET /api/guides`, `POST /api/guides/{id}/pause`, `POST /api/guides/{id}/remove`, `GET /api/guides/{id}/lifecycle`, `POST /api/guides/{id}/qr-download`: DB-backed in PostgreSQL mode for guide state and lifecycle audit events.
- `GET /api/customers`: DB-backed in PostgreSQL mode, with SQL `COUNT + LIMIT/OFFSET`.
- `POST /api/customers`: DB-backed in PostgreSQL mode for customer profile upserts.
- `GET /api/customers/{id}`: DB-backed in PostgreSQL mode, including tags, guide relations, deal amount, and customer event timeline.
- `POST/PATCH /api/customers/{id}/touch`, `/followups`, `/lead`, `/order`, `/lost`: DB-backed in PostgreSQL mode for customer operation and conversion actions.
- `POST/PATCH /api/customers/{id}/relations`, `/relations/{relationId}/set-main`, `/relations/{relationId}/end`: DB-backed in PostgreSQL mode for customer-guide relationship management.
- `GET/POST /api/events/inbox`: DB-backed in PostgreSQL mode, with idempotent inserts and worker processing.
- `GET/POST /api/tasks`, `GET /api/tasks/{id}`, `GET /api/tasks/{id}/items`: DB-backed in PostgreSQL mode.
- `POST /api/customers/batch-tags`: DB-backed in PostgreSQL mode for synchronous compatibility writes.
- `POST /api/customers/batch-tags-async`: DB-backed in PostgreSQL mode; worker persists tag bindings and customer events. Failed task items are retried and then moved to `dead_letter` after `SCRM_WORKER_MAX_ATTEMPTS`.
- `GET/POST /api/touches`: DB-backed in PostgreSQL mode for touch-rule templates.
- `GET/POST /api/groups`, `GET/POST /api/groups/mass`, `/welcomes`, `/sops`, `/calendar`, `/reminders`, `/tag-groups`: DB-backed in PostgreSQL mode for customer-group operation objects.
- `GET/POST /api/tags`, `PATCH /api/tags/{name}/toggle`, `PATCH /api/tags/{name}/rename`, `GET /api/tags/{name}/customers`: DB-backed in PostgreSQL mode, including customer counts and paginated customer lookups.
- `GET/POST /api/tag-groups`: DB-backed in PostgreSQL mode, including tag aggregation per group.
- `GET/POST /api/tag-rules/auto`, `/api/tag-rules/pre`: DB-backed in PostgreSQL mode for tag automation templates.
- `GET /api/summary`: DB-backed in PostgreSQL mode via `metric_snapshots`; missing snapshots are refreshed on demand.
- `POST /api/metrics/refresh`: refreshes the global dashboard metric snapshot for the requested date.
- `GET /api/metrics`: Prometheus-compatible runtime, HTTP, DB, cache, and worker backlog metrics.

Still memory-backed in PostgreSQL mode until later iterations:

- Enterprise WeChat callback verification and real external API calls.

Example local run:

```bash
SCRM_DATABASE_URL='postgres://scrm:scrm@127.0.0.1:5432/scrm?sslmode=disable' \
SCRM_AUTO_MIGRATE=true \
SCRM_DB_MAX_OPEN_CONNS=25 \
SCRM_DB_MAX_IDLE_CONNS=10 \
go run .
```

For 2,000-3,000 concurrent online users, size DB pools per API instance rather than globally. For example, four API instances with `SCRM_DB_MAX_OPEN_CONNS=25` can use up to 100 DB connections, before workers and admin tools are counted.

Health check:

```bash
curl http://127.0.0.1:8080/api/health
```

In memory mode, `database` is `disabled`. In PostgreSQL mode, the response includes pool counters such as open, in-use, and idle connections.

## Hot Read Cache

In PostgreSQL mode, the backend uses a short response cache for high-frequency read endpoints:

- `GET /api/summary`
- `GET /api/customers` and `GET /api/customers/{id}`
- `GET /api/tags`
- `GET /api/tags/{name}/customers`
- `GET /api/tag-groups`

Cache keys include the full request URI, so different pagination and filter parameters are isolated. Successful cacheable responses include:

```text
X-SCRM-Cache: MISS
X-SCRM-Cache: HIT
```

The cache is invalidated on relevant writes: customer upserts, event/task enqueue or processing, metric refresh, tag/tag-group mutations, and async batch-tag completion.

Cache backend selection:

- Without `SCRM_REDIS_ADDR`, the backend uses a local in-process cache. This is useful for single-instance demos and development.
- With `SCRM_REDIS_ADDR`, the backend uses Redis. This is the preferred multi-instance mode because cache entries and prefix invalidation are shared across API instances.

Local Redis runs on host port `16379` via Docker Compose:

```bash
npm run redis:up
SCRM_REDIS_ADDR=127.0.0.1:16379 npm run dev:backend:db
```

Redis prefix invalidation uses `SCAN` plus batched `DEL`, not `KEYS`, so it remains compatible with larger keyspaces. Keep the TTL short for operational pages; the cache is a pressure-relief layer, not the source of truth.

### Local PostgreSQL With Docker

From the repository root:

```bash
npm run db:up
npm run redis:up
npm run infra:up
```

Run the backend against PostgreSQL and apply migrations:

```bash
npm run dev:backend:db
npm run dev:backend:db:redis
```

Equivalent explicit command:

```bash
SCRM_DATABASE_URL='postgres://scrm:scrm@127.0.0.1:15432/scrm?sslmode=disable' \
SCRM_AUTO_MIGRATE=true \
go run .
```

The local Docker database listens on host port `15432` to avoid colliding with a system PostgreSQL on `5432`.

Useful commands:

```bash
npm run db:logs
npm run redis:logs
npm run db:down
```

## Main API Groups

- `GET /api/bootstrap`: full page data for the prototype.
- `GET /api/summary`: dashboard metrics.
- `GET /api/stores`, `PATCH /api/stores/{id}/config`: store list and private-domain config.
- `GET/POST /api/stores/{id}/guides`: store guide QR lifecycle.
- `POST /api/guides/{id}/pause`, `POST /api/guides/{id}/remove`, `GET /api/guides/{id}/lifecycle`: guide lifecycle actions.
- `POST /api/guides/{id}/qr-download`: return generated QR SVG payload.
- `POST /api/stores/{id}/handover/sync`, `POST /api/stores/{id}/handover/submit`: Enterprise WeChat leave/transfer handover loop.
- `GET/POST /api/customers`: customer pool.
- `POST /api/customers/batch-tags`: batch system tagging.
- `POST /api/customers/{id}/touch`, `/followups`, `/lead`, `/order`: customer touch and conversion loop.
- `POST /api/customers/{id}/relations`, `/relations/{relationId}/set-main`, `/relations/{relationId}/end`: customer-guide relationship management.
- `GET/POST /api/touches`: welcome, mass-send, SOP, and material rules.
- `GET/POST /api/groups`: customer group files.
- `GET/POST /api/groups/mass`, `/welcomes`, `/sops`, `/calendar`, `/reminders`, `/tag-groups`: customer group operation subpages.
- `GET/POST /api/tags`, `PATCH /api/tags/{name}/toggle`, `PATCH /api/tags/{name}/rename`, `GET /api/tags/{name}/customers`: tag lifecycle and tagged-customer lookup.
- `GET/POST /api/tag-groups`: tag group governance.
- `GET/POST /api/tag-rules/auto`, `/api/tag-rules/pre`: automatic and pre-tag rules.
- `GET/POST /api/events/inbox`: idempotent event inbox for Enterprise WeChat callbacks, scan events, and future async workers.
- `GET/POST /api/tasks`, `GET /api/tasks/{id}`, `GET /api/tasks/{id}/items`: async task tracking.
- `POST /api/customers/batch-tags-async`: create an async customer-tagging task.
- `POST /api/metrics/refresh`: refresh dashboard metric snapshots in PostgreSQL mode.
- `GET /api/metrics`: Prometheus-compatible operational metrics.

## Pagination Contract

List endpoints remain backward compatible for the current demo: without pagination parameters, they return a plain array.

When `page` / `pageSize` or `limit` / `offset` is provided, the same endpoints return a paginated envelope:

```json
{
  "data": [],
  "page": {
    "page": 1,
    "pageSize": 20,
    "total": 0,
    "totalPages": 0,
    "hasNext": false
  }
}
```

Rules:

- `pageSize` / `limit` must be between `1` and `200`.
- Use paginated mode for production pages and high-concurrency clients.
- Keep non-paginated mode only for the current prototype bootstrap and tiny dictionaries.

## Event Inbox

`POST /api/events/inbox` is the first implementation of the event write path. It stores the event quickly and uses `idempotencyKey` to avoid duplicate processing.

Example:

```bash
curl -X POST http://127.0.0.1:8080/api/events/inbox \
  -H 'Content-Type: application/json' \
  -d '{
    "eventId": "wecom-msg-001",
    "source": "wecom",
    "type": "external_contact.add",
    "objectId": "external-user-001",
    "payload": {"customer": "陈女士"}
  }'
```

The current demo keeps inbox events in memory. The persistent target is `backend/migrations/001_industrial_baseline.sql`, table `event_inbox`.

## Async Tasks

The worker processes queued event inbox records and queued task items on a short polling interval. In memory mode it uses process-local state; in PostgreSQL mode it uses `event_inbox`, `tasks`, and `task_items`.

PostgreSQL worker behavior uses short database updates and `FOR UPDATE SKIP LOCKED` for event claims, which keeps the pattern compatible with multiple API/worker instances. This establishes the same state machine expected from a future Redis Stream / RabbitMQ / Kafka worker:

```text
event_inbox: queued -> processed
task_items: queued -> running -> succeeded
task_items: queued -> running -> queued(retry) -> dead_letter
tasks: queued -> running -> completed | partial_failed | failed
```

`dead_letter` is intentionally explicit rather than hidden in `failed`: it means the item exceeded the configured retry budget and needs operator or reconciliation handling. `/api/metrics` exposes `scrm_task_items_dead_letter` so alerting can watch this count.

Create an async batch-tag task:

```bash
curl -X POST http://127.0.0.1:8080/api/customers/batch-tags-async \
  -H 'Content-Type: application/json' \
  -d '{
    "customerIds": ["c1", "c2"],
    "tags": ["高意向", "待跟进"],
    "operator": "运营"
  }'
```

Check task progress:

```bash
curl http://127.0.0.1:8080/api/tasks/{taskId}
```

The old `POST /api/customers/batch-tags` endpoint remains synchronous for prototype compatibility. Production clients should prefer the async endpoint for large batch operations. In PostgreSQL mode, both sync and async batch tagging write `customer_tags`, update `customers.updated_at`, and append a `customer_events` audit event.

## Customer Operations

In PostgreSQL mode, customer operation actions now update the customer master record and append auditable `customer_events` rows in one transaction. Customer detail reads assemble the profile, tag bindings, guide relations, conversion amount, and recent event timeline from persistent tables.

Useful checks:

```bash
curl -X POST http://127.0.0.1:8080/api/customers/c1/touch \
  -H 'Content-Type: application/json' \
  -d '{"method":"企微单聊待办","content":"发送试驾邀约"}'

curl -X POST http://127.0.0.1:8080/api/customers/c1/order \
  -H 'Content-Type: application/json' \
  -d '{"amount":"¥1,280","source":"导购跟进"}'

curl http://127.0.0.1:8080/api/customers/c1
```

## Tag Governance

In PostgreSQL mode, tag governance uses the same persistent `tag_groups`, `tags`, and `customer_tags` tables that async batch tagging writes to. This keeps the operational tag dictionary, customer tag counts, and tagged-customer drilldowns consistent under concurrent requests.

Useful checks:

```bash
curl 'http://127.0.0.1:8080/api/tags?page=1&pageSize=20'
curl 'http://127.0.0.1:8080/api/tag-groups?page=1&pageSize=20'
curl 'http://127.0.0.1:8080/api/tags/高意向/customers?page=1&pageSize=20'
```

## Metric Snapshots

In PostgreSQL mode, `GET /api/summary` reads the global dashboard from `metric_snapshots`. If the current date has no complete snapshot, the backend computes a lightweight snapshot once, stores it, and returns the same response shape the prototype expects:

- `todayPool`
- `attributionRate`
- `pending`
- `touchRate`
- `trend`
- `suggestions`

Refresh the snapshot explicitly:

```bash
curl -X POST 'http://127.0.0.1:8080/api/metrics/refresh'
curl -X POST 'http://127.0.0.1:8080/api/metrics/refresh?date=2026-06-26'
```

For production, put this refresh behind an admin permission and run it from a scheduled worker. User-facing dashboard requests should read snapshots instead of recalculating large operational tables on every page load.

## Observability

`GET /api/metrics` exposes Prometheus-compatible text metrics without adding a separate dependency. It includes:

- Request totals by method, normalized route, and status class.
- Request duration histogram buckets in milliseconds.
- Error counters.
- Runtime goroutines and heap allocation.
- PostgreSQL pool usage.
- Redis/local cache backend metrics.
- Event inbox and task-item backlog gauges.

Example:

```bash
curl http://127.0.0.1:8080/api/metrics
```

Dynamic IDs are normalized in metric labels, for example `/api/customers/c1` becomes `/api/customers/{id}`. This avoids high-cardinality labels during production traffic.

## Load Testing

The repository includes a lightweight Node-based load test for the hot read paths:

```bash
npm run perf:smoke
```

Custom run:

```bash
BASE_URL=http://127.0.0.1:8080 \
DURATION_SECONDS=60 \
CONCURRENCY=200 \
TARGET_RPS=1000 \
npm run perf:load
```

The script prints total requests, QPS, failure rate, P50/P95/P99/max latency, response status distribution, and `X-SCRM-Cache` hit rate. Use it after starting PostgreSQL and Redis:

```bash
npm run infra:up
npm run dev:backend:db:redis
```

For the 2,000-3,000 online-user target, do not map online users directly to concurrency. Start with 100-300 concurrent workers for API pressure, inspect P95, DB connection wait count, Redis hit rate, and worker backlog, then scale up gradually.

`TARGET_RPS` is optional. Without it, the script runs as fast as possible and will usually hit the API rate limiter unless `SCRM_RATE_LIMIT_RPS` is raised or disabled for a controlled test.

## Database Baseline

`migrations/001_industrial_baseline.sql` defines the initial PostgreSQL schema for the industrial version:

- Customer core, identities, attribution, and staff relations.
- Store code lifecycle.
- Store profiles, store private-domain config, guide profiles, guide handover workflow, and guide lifecycle events.
- Tags and customer tag bindings.
- Customer-group operation objects, touch-rule templates, and tag-rule templates.
- Customer operation/conversion events and idempotent event inbox.
- Async task and task-item tracking.
- Dashboard/report metric snapshots.
- Demo seed data in `migrations/002_seed_demo.sql`; seed inserts use `DO NOTHING` on conflicts so startup migrations do not overwrite operational changes.

## Current Boundary

- Default mode is still process-local and resets on restart.
- PostgreSQL connection, migration, health support, store/guide repository paths, store handover workflow, customer repository paths and customer operation actions, customer-group operation objects, touch-rule templates, tag-rule templates, event inbox, async task paths, tag governance paths, dashboard metric snapshots, Redis/local hot-read cache, Prometheus metrics, and load-test tooling are available.
- Auth, organization permission, Enterprise WeChat callbacks, business timezone normalization, Redis Streams/MQ dead-lettering, and deployment automation are intentionally left for the next backend iterations.
