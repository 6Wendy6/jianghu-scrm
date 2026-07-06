# SCRM MVP Acceptance Checklist

本文档用于验收当前 SCRM MVP 是否形成业务闭环。它不是完整自动化测试用例，而是产品、测试、业务演示和研发联调都能使用的手工验收清单。

验收约定：

- 优先使用前端原型 `frontend/public/prototype.html` 演示业务链路。
- 后端接口默认指向当前运行的 API，例如 `http://127.0.0.1:18082` 或 `.env` 中的 `SCRM_API_ADDR`。
- PostgreSQL 行为验收需要先配置 `DATABASE_URL` 或 `SCRM_DATABASE_URL`，并执行 `npm run db:migrate`、`npm run db:seed`。
- 不重置现有数据库，不删除 demo seed。
- 如果某项仍属于 MVP 原型而非后端持久化，需要在结果中标注“原型通过 / 后端待接入”。

## 验收记录模板

| 字段 | 记录 |
| --- | --- |
| 验收人 |  |
| 验收日期 |  |
| 前端地址 |  |
| 后端地址 |  |
| 运行模式 | memory / postgres |
| 数据库 | 未启用 / 已连接 |
| 迁移版本 |  |
| 总体结论 | 通过 / 有条件通过 / 失败 |

## 场景 1：总部查看全局资料缺失并分派补全任务

### 验收目标

验证总部能发现门店、店长、导购、客户资料缺失，并把补全事项分派到门店处理。

### 前置条件

- 前端原型可访问。
- 已有资料补全中心 demo 数据。
- 以总部模式进入系统。

### 涉及页面

- 总部工作台。
- 资料补全中心。
- 异常监控。

### 涉及接口

当前主要为前端原型状态；后续后端化建议接口：

- `GET /api/data-quality/issues`
- `POST /api/data-quality/issues/{id}/assign`
- `POST /api/data-quality/issues/{id}/resolve`

### 操作步骤与预期结果

| 步骤 | 操作 | 预期结果 | 结果 |
| --- | --- | --- | --- |
| 1 | 进入总部模式 | 可看到总部视角导航和全局数据 | 通过 / 失败 |
| 2 | 打开资料补全中心 | 展示门店、店长、导购、客户资料问题 | 通过 / 失败 |
| 3 | 查看 P0/P1 缺失 | P0/P1 问题有优先级、对象、门店、字段说明 | 通过 / 失败 |
| 4 | 按门店筛选 | 只显示所选门店相关缺失 | 通过 / 失败 |
| 5 | 分派给门店 | 问题状态或处理日志显示已分派 | 通过 / 失败 |
| 6 | 切换门店模式 | 门店视角能看到总部派发事项 | 通过 / 失败 |
| 7 | 补全资料 | 缺失字段被补齐，问题状态变更 | 通过 / 失败 |
| 8 | 返回总部模式查看 | 问题数量减少或状态变为已解决 | 通过 / 失败 |

### 验收结论

| 项目 | 记录 |
| --- | --- |
| 是否通过 |  |
| 失败步骤 |  |
| 备注 |  |

## 场景 2：新客户进入后完成首次跟进

### 验收目标

验证新客户进入客户运营中心后，导购可以查看客户详情、完成首次跟进、写入时间线，并推动生命周期流转。

### 前置条件

- 后端服务可用。
- 客户运营中心可加载客户数据。
- DB 模式下已执行迁移和 seed；memory 模式下使用内置 demo 数据。

### 涉及页面

- 客户运营中心。
- 客户详情抽屉。
- SOP 任务区域。

### 涉及接口

- `GET /api/customer-ops/bootstrap`
- `GET /api/customer-ops/customers`
- `GET /api/customers/{id}`
- `POST /api/customers/{id}/followups`
- `POST /api/customer-ops/customers/{id}/stage`
- `POST /api/sop-tasks/{id}/complete`

### 操作步骤与预期结果

| 步骤 | 操作 | 预期结果 | 结果 |
| --- | --- | --- | --- |
| 1 | 进入客户运营中心 | 客户列表和任务列表正常加载 | 通过 / 失败 |
| 2 | 打开客户详情 | 看到客户阶段、来源、门店、导购、标签和时间线 | 通过 / 失败 |
| 3 | 查看首次跟进 SOP 任务 | 能看到待处理任务、截止时间、负责人 | 通过 / 失败 |
| 4 | 导购写跟进记录 | 跟进记录保存成功 | 通过 / 失败 |
| 5 | 查看客户时间线 | 时间线增加跟进事件 | 通过 / 失败 |
| 6 | 推动客户阶段 | 客户阶段流转为跟进中或高意向 | 通过 / 失败 |
| 7 | 完成 SOP 任务 | 任务状态变为已完成 | 通过 / 失败 |
| 8 | 刷新页面 | DB 模式下跟进、阶段、任务状态仍保留 | 通过 / 失败 |

### 验收结论

| 项目 | 记录 |
| --- | --- |
| 是否通过 |  |
| 失败步骤 |  |
| 备注 |  |

## 场景 3：客户标签和画像维护

### 验收目标

验证客户标签可以写入、刷新后保留，并能通过客户画像和筛选支撑后续运营。

### 前置条件

- 客户运营中心可访问。
- DB 模式下建议使用 `backend/seeds/customer_ops_demo_seed.sql` 初始化演示数据。

### 涉及页面

- 客户运营中心。
- 客户详情抽屉。
- 标签筛选区域。

### 涉及接口

- `GET /api/customers/{id}`
- `POST /api/customers/{id}/tags`
- `GET /api/customer-ops/customers?tag=...`
- `GET /api/tags`

### 操作步骤与预期结果

| 步骤 | 操作 | 预期结果 | 结果 |
| --- | --- | --- | --- |
| 1 | 打开客户详情 | 客户当前标签可见 | 通过 / 失败 |
| 2 | 添加客户标签 | 保存成功，标签立即显示 | 通过 / 失败 |
| 3 | 刷新页面 | DB 模式下标签仍然存在 | 通过 / 失败 |
| 4 | 查看时间线 | 时间线记录标签变化 | 通过 / 失败 |
| 5 | 回到客户列表 | 可按标签筛选客户 | 通过 / 失败 |
| 6 | 查看标签统计 | 标签客户数或标签列表可反映变化 | 通过 / 失败 |

### 验收结论

| 项目 | 记录 |
| --- | --- |
| 是否通过 |  |
| 失败步骤 |  |
| 备注 |  |

## 场景 4：SOP 任务逾期进入异常监控

### 验收目标

验证 SOP 逾期可以被识别为异常，并在处理后更新异常状态和保留记录。

### 前置条件

- 存在已逾期或可构造逾期的 SOP 任务。
- 客户运营中心和异常监控可访问。

### 涉及页面

- 客户运营中心。
- 异常监控。
- 客户详情抽屉。

### 涉及接口

- `GET /api/sop-tasks`
- `POST /api/sop-tasks`
- `POST /api/sop-tasks/{id}/complete`
- `GET /api/operation-exceptions`
- `POST /api/operation-exceptions/{id}/resolve`

### 操作步骤与预期结果

| 步骤 | 操作 | 预期结果 | 结果 |
| --- | --- | --- | --- |
| 1 | 创建或使用已逾期任务 | 任务显示为待处理且超过截止时间 | 通过 / 失败 |
| 2 | 执行逾期扫描或打开异常监控 | 异常监控显示 SOP 逾期 | 通过 / 失败 |
| 3 | 打开异常详情 | 能看到客户、门店、导购、任务和处理建议 | 通过 / 失败 |
| 4 | 处理异常或完成关联任务 | 异常状态更新为已处理/已解决 | 通过 / 失败 |
| 5 | 查看处理记录 | 异常处理记录保留 | 通过 / 失败 |
| 6 | 查看客户时间线 | 与客户相关的处理动作有记录 | 通过 / 失败 |

### 验收结论

| 项目 | 记录 |
| --- | --- |
| 是否通过 |  |
| 失败步骤 |  |
| 备注 |  |

## 场景 5：总部/门店/导购权限隔离

### 验收目标

验证不同角色只能看到自己范围内的数据，越权访问客户详情时返回 `403` 或业务错误。

### 前置条件

- 后端服务可用。
- DB 模式下有多门店、多导购客户数据。
- 可通过请求头模拟角色和范围。

### 涉及页面

- 客户运营中心。
- 客户详情。
- 异常监控。

### 涉及接口

- `GET /api/customer-ops/customers`
- `GET /api/customers/{id}`
- `GET /api/sop-tasks`
- `GET /api/operation-exceptions`

### 请求头示例

```bash
X-SCRM-Role: headquarters_admin
X-SCRM-Region-ID: region-south
X-SCRM-Store-ID: store-nanshan
X-SCRM-Guide-ID: guide-jiang
```

### 操作步骤与预期结果

| 步骤 | 操作 | 预期结果 | 结果 |
| --- | --- | --- | --- |
| 1 | 总部视角访问客户列表 | 能看全部或授权范围内客户 | 通过 / 失败 |
| 2 | 门店视角访问客户列表 | 只看到本门店客户 | 通过 / 失败 |
| 3 | 导购视角访问客户列表 | 只看到自己负责客户 | 通过 / 失败 |
| 4 | 导购访问自己的客户详情 | 返回 200，详情可见 | 通过 / 失败 |
| 5 | 导购访问无权限客户详情 | 返回 403 或业务错误 | 通过 / 失败 |
| 6 | 门店查看任务和异常 | 只看到本门店任务和异常 | 通过 / 失败 |

### 验收结论

| 项目 | 记录 |
| --- | --- |
| 是否通过 |  |
| 失败步骤 |  |
| 备注 |  |

## 场景 6：系统状态检查

### 验收目标

验证本地、演示、测试环境可以通过接口识别当前运行模式、数据库连接、迁移版本和缺失表状态。

### 前置条件

- 后端服务已启动。
- 如果测试 PostgreSQL 模式，已配置 `DATABASE_URL` 或 `SCRM_DATABASE_URL`。

### 涉及页面

- 前端原型右下角 dev 状态提示。

### 涉及接口

- `GET /api/health`
- `GET /api/system/status`

### 操作步骤与预期结果

| 步骤 | 操作 | 预期结果 | 结果 |
| --- | --- | --- | --- |
| 1 | 调用 `/api/health` | 返回 `status=ok`，包含 `mode` 和 `storage` | 通过 / 失败 |
| 2 | 调用 `/api/system/status` | 返回系统状态、DB 状态、迁移版本 | 通过 / 失败 |
| 3 | 在 memory 模式检查 | `database.connected=false` 或 disabled，系统仍可运行 | 通过 / 失败 |
| 4 | 在 postgres 模式检查 | `database.connected=true`，显示 pool 和 migration version | 通过 / 失败 |
| 5 | 检查缺失表 | 正常情况下 `missingRequiredTables=[]` | 通过 / 失败 |
| 6 | 打开前端原型 | 右下角状态提示显示当前模式和迁移信息 | 通过 / 失败 |

## 场景 7：企业微信集成中心底座检查

### 验收目标

验证真实企业微信接入前，系统可以检查配置、access_token 缓存、权限、错误解释、重试任务和集成状态。

### 前置条件

- 后端以 PostgreSQL 模式启动。
- 已执行 `npm run db:migrate`。
- 可先不配置真实企业微信 secret；未配置时应返回 attention 和下一步建议。

### 涉及页面

- 总部工作台。
- 异常监控。
- 系统状态或接口调试页面。

### 涉及接口

- `GET /api/scrm/wecom/config`
- `PUT /api/scrm/wecom/config`
- `POST /api/scrm/wecom/test-token`
- `POST /api/scrm/wecom/permission-check`
- `GET /api/scrm/wecom/permission-checks`
- `GET /api/scrm/wecom/status`
- `GET /api/scrm/wecom/error-dictionary`
- `GET /api/scrm/wecom/doctor`
- `GET /api/scrm/wecom/retries`
- `GET /api/system/status`

### 操作步骤与预期结果

| 步骤 | 操作 | 预期结果 | 结果 |
| --- | --- | --- | --- |
| 1 | 调用 `/api/scrm/wecom/status` | 返回配置脱敏信息、token 状态、资产数量、retry 摘要和 nextSteps | 通过 / 失败 |
| 2 | 未配置企微时查看结果 | `status=attention`，提示保存 corpId/secret/callback 配置 | 通过 / 失败 |
| 3 | `PUT /api/scrm/wecom/config` 保存 corpId、secret、agentId 和启用状态 | 返回 `status=ok`，再次 `GET` 时只看到脱敏和已配置标记，不返回 secret 明文 | 通过 / 失败 |
| 4 | `POST /api/scrm/wecom/test-token` 测试单个或全部 token | 成功时只返回 token 类型、状态、过期时间；失败时返回结构化错误码和建议 | 通过 / 失败 |
| 5 | `POST /api/scrm/wecom/permission-check` | 记录 contact/customer/app token、部门读取、成员读取、客户联系基础权限；暂未做的同步项标记 planned/not_checked | 通过 / 失败 |
| 6 | `GET /api/scrm/wecom/permission-checks` | 返回最近权限检测历史，包含 status、errcode、localCode、suggestion、checkedAt | 通过 / 失败 |
| 7 | 制造 60011、60020 或 token 错误 | 返回本地错误码，并在异常监控生成/更新企微接入异常 | 通过 / 失败 |
| 8 | 调用 `/api/scrm/wecom/error-dictionary` | 返回常见错误码、业务含义和处理建议 | 通过 / 失败 |
| 9 | 调用 `/api/scrm/wecom/doctor` | 返回配置、token、get_contact_way、update_contact_way dry-run 检查结果 | 通过 / 失败 |
| 10 | 调用 `/api/system/status` | 返回 `wecom` 摘要，包含 configured、tokens、permissionChecks、assets、retry | 通过 / 失败 |

### 验收结论

| 项目 | 记录 |
| --- | --- |
| 是否通过 |  |
| 失败步骤 |  |
| 备注 |  |

### 验收结论

| 项目 | 记录 |
| --- | --- |
| 是否通过 |  |
| 失败步骤 |  |
| 备注 |  |

## 验收命令

基础后端测试：

```bash
npm run test:backend
```

PostgreSQL 行为测试：

```bash
SCRM_TEST_DATABASE_URL=postgres://scrm:scrm@127.0.0.1:15432/scrm_test?sslmode=disable npm run test:backend:db
```

前端构建：

```bash
npm run build
```

系统状态：

```bash
curl http://127.0.0.1:8080/api/health
curl http://127.0.0.1:8080/api/system/status
```

## 最终签收

| 项目 | 结论 |
| --- | --- |
| 产品主线是否清晰 | 通过 / 有条件通过 / 失败 |
| 核心业务闭环是否成立 | 通过 / 有条件通过 / 失败 |
| DB 持久化是否满足 MVP 验收 | 通过 / 有条件通过 / 失败 |
| 权限隔离是否满足 MVP 验收 | 通过 / 有条件通过 / 失败 |
| 演示是否稳定 | 通过 / 有条件通过 / 失败 |
| 下一阶段是否可进入企微集成中心 | 是 / 否 |
