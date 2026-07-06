# SCRM Test Data Safety

本文档定义 SCRM 的测试数据隔离和清理规则。原则只有一句话：

测试数据必须可识别、可隔离、可回滚、可清理；宁愿少测，也不能污染真实客户、门店、导购和企微数据。

## 1. 数据库隔离

自动化测试优先使用独立测试库或临时 PostgreSQL 容器：

```text
临时 PostgreSQL 容器
  -> migrate
  -> seed / create test data
  -> run tests
  -> cleanup report
  -> docker rm -f 临时容器
```

测试脚本只能默认读取 `SCRM_TEST_DATABASE_URL`。不要把 E2E 测试数据写入 `DATABASE_URL` 或 `SCRM_DATABASE_URL` 指向的开发/演示/生产库。

禁止事项：

- 不允许删除、重置或清空现有 `15432` PostgreSQL volume。
- 不允许对真实 `DATABASE_URL` 直接写入 E2E 测试数据。
- 不允许执行无 `WHERE` 条件的 `DELETE`。
- 不允许执行 `TRUNCATE`、`DROP` 清理业务表。

## 2. test_run_id 规范

每次 E2E 或人工验收测试必须生成唯一 `test_run_id`：

```text
e2e_YYYYMMDD_HHMMSS
```

所有测试数据必须带 `test_run_id` 或包含该 ID 的前缀：

```text
[E2E_TEST_e2e_20260706_153000] 李女士
[E2E_TEST_e2e_20260706_153000] 杭州测试店
source_channel = E2E_TEST_e2e_20260706_153000
remark = E2E_TEST_CREATED_BY_AUTOMATION e2e_20260706_153000
```

覆盖范围包括：

- 测试客户
- 测试门店
- 测试店长
- 测试导购
- 测试标签和标签组
- 测试跟进记录
- 测试 SOP 任务和日志
- 测试异常
- 测试企业微信配置、token、权限检测、活码包装、分配和回调记录

不要只写通用 `[E2E_TEST]` 前缀。通用前缀无法区分本次测试和历史测试，清理脚本默认不会按通用前缀删除。

## 3. 清理脚本

脚本位置：

```bash
scripts/e2e-cleanup.mjs
```

dry-run：

```bash
SCRM_TEST_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable \
  npm run e2e:cleanup:dry-run -- --test-run-id=e2e_20260706_153000
```

apply：

```bash
SCRM_TEST_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable \
  npm run e2e:cleanup:apply -- --test-run-id=e2e_20260706_153000
```

脚本行为：

- 默认 dry-run。
- 只读取 `SCRM_TEST_DATABASE_URL` 或显式 `--database-url`。
- 拒绝连接 `15432`。
- 拒绝不含 `test` / `e2e` / `ci` 的数据库名，除非显式传 `--allow-shared-db`。
- apply 前会输出 dry-run 数量。
- apply 后会再次查询残留。
- 如果残留数量不为 0，脚本以失败退出。

如果本机没有 `psql`，脚本会使用 `postgres:16-alpine` Docker 镜像中的 `psql` 客户端执行 SQL。

## 4. 清理顺序

清理必须从子表到主表：

1. 异常记录、时间线、任务日志。
2. SOP 任务、跟进记录、生命周期日志。
3. 客户标签关系、客户事件、客户身份、客户导购关系、客户归因。
4. 客户。
5. 标签、标签组。
6. 导购事件、导购、门店。
7. SCRM 企微包装相关的重试、分配、回调、绑定、导购池、门店。
8. 企业微信测试配置、token、权限检测和旧版企微 pilot 表。

清理条件必须包含本次 `test_run_id`。

## 5. 测试报告必须包含

每次 E2E 或人工验收报告必须记录：

- `test_run_id`
- 是否使用临时数据库
- 是否触碰 `15432` 数据卷
- 创建数据统计
- cleanup dry-run 统计
- cleanup apply 统计
- cleanup 后残留数据统计
- 如有残留，列出原因和手动清理 SQL；不得自动删除非测试数据

## 6. 推荐命令模板

临时数据库测试建议：

```bash
TEST_RUN_ID=e2e_$(date +%Y%m%d_%H%M%S)
docker run -d --name jianghu-scrm-postgres-e2e \
  -e POSTGRES_USER=scrm \
  -e POSTGRES_PASSWORD=scrm \
  -e POSTGRES_DB=scrm_test \
  -p 15433:5432 postgres:16-alpine

DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable \
SCRM_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable \
  npm run db:migrate
DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable \
SCRM_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable \
  npm run db:seed

# run e2e tests with TEST_RUN_ID

SCRM_TEST_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable \
  npm run e2e:cleanup:dry-run -- --test-run-id=$TEST_RUN_ID
SCRM_TEST_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15433/scrm_test?sslmode=disable \
  npm run e2e:cleanup:apply -- --test-run-id=$TEST_RUN_ID

docker rm -f jianghu-scrm-postgres-e2e
```
