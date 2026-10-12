# ESPulse — 项目约定（Copilot 项目记忆）

## 工作方式（最重要，先读这一条）

- **以「快速、正确的编码」为唯一交付目标。** 写完即交付，不要自作主张加验证环节。
- **不要跑构建、测试、lint、类型检查，也不要用浏览器调试 UI 问题。** 这些既慢又费 token，收益由用户自己验证 —— **UI 一律由用户亲自验证**。
- **忽略警告类问题**：未使用的导入 / 变量、lint 提示、`vue-tsc` 的 `TS6133` 之类，都不要修、不要提。
- **唯一需要自查的是「断链」**：删文件 / 改导出后，用 `grep` 之类最廉价的手段确认没有别处还在引用（import 指向已删除文件、使用了已删除的符号）。这属于「语法 / 引用错误」，不属于上面被禁止的验证。
- 需要澄清时直接问用户，不要靠大范围探索去猜。
- 用户明确要求跑命令（构建 / 测试 / 浏览器）时才跑，并挑**最小的那一条**，不要重复跑全量。

## 项目是什么

ESPulse = Go + Vue 3 的轻量 Elasticsearch 集群管理工具，目标替代依赖 JDK 的 Cerebro。
交付形态：单个 Go 二进制（前端 `embed.FS` 内嵌）；采集后端按需共享（5s 间隔 + 失败指数退避 + 快照落库）。

## 页面骨架（改前端前必须知道）

| 路由 | 页面 | 职责 |
|---|---|---|
| `/` | `views/ClusterHub.vue`（外壳 `layouts/MainLayout.vue`） | 集群管理 + 浏览：卡片墙、增删改、0 集群引导 |
| `/workbench/:id` | `views/Workbench.vue` | **集群唯一落地页**：指标条 → 活动区 → 统一网格（索引 / 节点双视角）→ Dev Console 模式 |

- **已删除、不要再引入**：`ClusterOverview.vue` / `ClusterIndices.vue` / `ClusterShards.vue` / `DevConsole.vue` / `composables/useViewFilters.ts`；旧路径 `/cluster/:id/*` 只在路由里做重定向。
- 集群切换器（`components/ClusterSwitcher.vue`）统一把用户带向 `/workbench/:id`。
- 结构细节见 [ARCHITECTURE.md](../ARCHITECTURE.md) §2，交互规则见 [PRD.md](../PRD.md) §2.5。

## 代码约定

- 注释与文档全部用**中文**；注释只写「为什么」，不写「做了什么」。
- 前端：Vue 3 `<script setup>` + TypeScript + UnoCSS + Naive UI；工作台专属样式写在 `styles/workbench.less`（作用域 `.esp-wb`），不要影响 UnoCSS 页面。
- 后端：Go + Gin，HTTP 客户端走 `internal/es/client.go` 的 `SharedClient` / `ProbeClient` / `CatalogClient`；ES 接口只用跨版本稳定的 `_cluster/health`、`_cat/*?format=json`。
- 前端文案先落 `i18n/locales/{zh-CN,en-US}.ts`，不要硬编码在模板里。

## 提交约定

- **一次提交只做一件事**：如果改动横跨多个功能 / 模块（如「后端加了接口 + 前端改了页面 + 顺手修了个 bug」），**拆成多次提交**，每次只包含同一件事的改动；不要把一个功能的全套改动打散到多次提交，也不要一次提交塞进多件事。
- **提交信息全英文**（含正文）：项目是开源的，将来可能有协作者，历史对所有人都要能读。首行 `type(scope): subject`；正文只写**必要的补充**，用 Markdown 的 `-` 列表列点，一条一句，**一般不超过 3 条**。改动本身能从 diff 看出来的，不要复述 —— 正文留给「为什么这么改」「有什么副作用 / 影响范围」。
- 注释与文档仍用中文；正文里引用中文文档时写 `PRD §2.5` 这类章节号即可，不要把中文段落整段搬进提交信息。
- 需要在同一批改动里说明的背景（如文档转正、约定变更），合并进相关那一次提交的正文，不要单独开一次「顺便更新文档」的提交。
- **不要加任何多余信息**：`Co-authored-by: Copilot` / `Generated with …` 这类工具署名 trailer 一律不写，末尾也不留空行。只有用户明确要求时才加 trailer。

## 文档体系（改动要同步）

- [PRD.md](../PRD.md) —— 产品原则与阶段规划；[ARCHITECTURE.md](../ARCHITECTURE.md) —— 结构与接口；[TASKS.md](../TASKS.md) —— 任务看板；[CHANGELOG.md](../CHANGELOG.md) —— 每次改动追加到 `## [Unreleased]` 下当天日期的小节。
- 行为 / 架构 / 路由发生变化时，至少同步 TASKS + CHANGELOG；涉及原则与页面骨架时同步 PRD；涉及目录或 API 时同步 ARCHITECTURE。
- **规划转正的改动**（如原型页转为正式页面）必须在 PRD 里把「原型 / 并存」的表述一并改掉，不能只加新段落。
