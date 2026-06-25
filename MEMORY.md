# AI SCRM Memory

## Project Snapshot

- Project name: AI SCRM
- Workspace root: `/Users/chaoyun/Desktop/AI SCRM`
- Initial memory date: 2026-06-24
- Apparent stack: Markdown product documents, static HTML prototype, JSON page inventories, screenshot assets.

## Known Files And Assets

- `SCRM产品方案.md`: Product plan document.
- `好客私域通业务模块分析.md`: Detailed business-module analysis based on screenshots, page trees, page inventory, and manual subpage captures.
- `线下门店私域MVP点击原型.html`: Static clickable prototype.
- `线下门店私域实现方案.md`: Implementation plan document.
- `scrm_page_inventory.json`: Page inventory data.
- `scrm_manual_subpages_v2.json` and `scrm_manual_subpages_remaining.json`: Manual subpage data.
- `screenshots/`, `page_states/`, `采集截图与页面树/`: Screenshot and page-state assets.

## Decisions

- 2026-06-24: Created project-local `AGENTS.md` and `MEMORY.md` because the workspace lacked both files and `~/.codex/templates/` did not exist.
- 2026-06-24: Local static preview runs from the project root on `http://127.0.0.1:8765/`; prototype entry is `/线下门店私域MVP点击原型.html`.
- 2026-06-24: `好客私域通业务模块分析.md` treats the product as a full Enterprise WeChat SCRM covering acquisition attribution, customer operation, sales conversion, service, risk control, data analytics, and enterprise administration. Pages with likely capture/navigation anomalies are explicitly marked as needing recheck.
- 2026-06-24: Rebuilt `线下门店私域MVP点击原型.html` around the current offline-store private-domain scope: dashboard metrics, store private-domain config, store-staff codes, material-code assignment, attribution lock, identity merge, customer handover queue, guide workbench/sidebar, official WeCom touchpoints, group operations, and tag/rule settings.
- 2026-06-24: Audit finding for the prototype: first-level modules cover the current-period closed loop, but many second-level drawers are still explanatory stubs. Completion should be judged by whether each action changes visible state or returns to the next step in the offline-store private-domain workflow, not merely by having an entry button.
- 2026-06-24: Reworked the `标签与规则` page from a mixed parameter table into a customer tag management workspace modeled on Haoke SCRM logic: tag groups, tag list, WeCom tag sync, usage entry points, automatic tagging rules, lifecycle/pre-tag rules, and system parameters separated into tabs.
- 2026-06-24: Added a lightweight in-page state layer to the HTML prototype for priority second-level closed loops: adding store reception staff creates a visible staff/code row, saving store private-domain config refreshes the config card, submitting handover updates customer ownership and queue status, new welcome/SOP rules append to touch-operation lists, and customer detail drawers now switch by selected row.
- 2026-06-24: Added a consistent page guide card to every first-level page in the prototype. Each guide explains the page task, the first recommended operation, and where to go next, so users can understand the offline-store private-domain workflow while still using dense operational screens.
- 2026-06-24: Upgraded the prototype page hierarchy from simple guide cards to a flow-based object/action/result layout. Store config, customer acquisition codes, customer pool, guide workbench, touch operations, and customer group operations now show their position in the offline-store private-domain closed loop, the page objects, recommended action, and expected result before the dense tables/actions.
- 2026-06-24: Removed the global workflow guide cards and the dashboard goal notice from the prototype because they added visual noise after the page-level hierarchy was clear enough. Reworked the tag/rule drawers into operational flows: create tag plus optional auto-tag rule on one page, auto-tag impact preview with visible calculation, selectable store scope, tag-group creation with label association/order controls, and WeCom tag sync with system-tag mapping.
- 2026-06-24: Rebuilt the `标签与规则` page as a beginner-friendly workbench instead of a tag inventory page. The page now leads with a three-step operation order, shows the data flow from sources to tag processing to business usage, separates tag dictionary, auto-tag rules, usage relationships, and system parameters into clear tabs, and replaces vague jump cards with explicit usage scenarios explaining customer-pool filtering, mass-send targeting, code-based auto-tagging, and guide workbench usage.
- 2026-06-24: Created separate worktree `/Users/chaoyun/Desktop/AI SCRM-tag-rules-redesign` on branch `codex/tag-rules-screenshot-redesign` for a full `标签与规则` redesign without overwriting the current workspace. In this worktree, the page is rebuilt to match the provided Haoke-style screenshot: top tabs `客户标签 / 标签统计 / 管理规则`, toolbar controls for adding tag groups, help, department scope, and WeCom sync, plus row-based tag groups with visible scope, tag chips, add, and edit actions.
- 2026-06-24: Corrected the tag-group model in the screenshot-style redesign: `标签统计` must display both tag and tag-group fields, and the edit drawer for a tag group should edit group-level fields (name, visible scope, departments, owner, display order, group tags, and business usage), not a generic tag-management table.
- 2026-06-24: Closed the pre-tag rule save loop in the screenshot-style tag redesign. `管理规则` now separates automatic tagging rules from pre-tag rules; saving a pre-tag rule returns to the management tab, shows a top success notice, and writes the saved activity/tag/period into the visible `预打标签规则` table.
- 2026-06-24: Updated the store/code workflow per product review: `客户入池管理` is renamed to `客户管理` and now shows customer search/list data instead of code lists; store detail `门店活码生命周期` has search and pagination controls, removes redundant closed-loop copy and “查看全部活码”, adds `查看客户` actions that jump to filtered customer data, and handover now requires syncing Enterprise WeChat leave/transfer data before showing the handover list.
- 2026-06-24: Customer detail drawer was widened and changed from a read-only profile into an operable customer-guide relationship panel. It now uses neutral section titles (`客户详情`, `来源归属`, `服务归属`, `关联导购信息`), tracks guide relationship start/end dates, supports adding/ending guide links, setting/transferring the main follow-up guide, and adding follow-up records with visible timeline feedback.
- 2026-06-24: Simplified the store module wording to plain `门店` and moved `处理导购交接` next to `添加接待导购`. Customer management now leads with batch actions, changes row operations to `客户详情 / 触达客户`, and single-customer touch creates a task plus timeline entry.
- 2026-06-24: Customer detail timeline entries should be auditable: every customer-guide relationship, follow-up, touch, or handover action records an exact timestamp, related customer/guide/operator, and a short result description.
- 2026-06-24: Customer group operations now follow the Haoke-style secondary menu structure under `客户群运营`: `客户群管理 / 入群欢迎语 / 客户群群发 / 群SOP / 群日历 / 客户群提醒 / 客户群标签`. Each child page must expose its own object list and save loop instead of being a static explanation page.
- 2026-06-24: Closed the branch-analysis gaps in the tag-rules redesign worktree by adding final override renderers for customer management and customer group operations. Customer management now has batch actions plus row-level customer detail/touch actions, and customer group secondary pages keep visible state after creating groups, mass sends, welcome/SOP/calendar/reminder rules, and group tag groups.
- 2026-06-25: Based on `好客私域通客户转化逻辑分析.md`, the MVP branch now covers the missing conversion loop after private-domain acquisition: behavior radar creates high-intent signals and guide tasks, customer management shows intent score and sales stage, customer detail can create leads/opportunities and record orders/payments, and conversion results flow back into customer tags, lifecycle, timelines, sales pipeline, and customer-group contribution.
- 2026-06-25: The tag-rules redesign worktree is published to GitHub at `https://github.com/6Wendy6/jianghu-scrm` on branch `codex/tag-rules-screenshot-redesign`.
- 2026-06-25: The Go backend now uses an in-memory business API layer as the first implementation target. It covers the prototype closed loops for store private-domain config, guide QR lifecycle, handover sync/submit, customer relations/touch/conversion, customer group operations, and tag/rule governance before introducing a database.

## Open Questions

- None currently.
