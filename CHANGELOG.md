# CHANGELOG — ESPulse 变更日志

> 记录项目的每一个足迹。

---

## [Unreleased]

### 2026-10-02
- **Docs**: 明确 Overview 与分片分布的职责边界（PRD §6.1 / §6.2）—— Overview 定位为「状态仪表盘 + 问题清单」的分诊台；Cerebro 式「节点 × 索引」分片矩阵下沉为独立的分片分布页，支持「按索引 / 按节点」视角与过滤。
- **Docs**: 确立采集模型（PRD §6.3 / ARCHITECTURE §4）—— 由「无条件定时全量采集」改为「按需激活的共享采集」：按 `(集群, kind)` 维护共享缓存，单飞去重保证 N 个并发用户只打一次 ES，`lastAccess` 空闲淘汰保证无人查看时停止采集；活跃信号采用轮询心跳，`_cat/shards` 等重接口在页面活跃期间随其他 kind 定时共享刷新。
- **Docs**: TASKS 看板同步细化集群总览子项与分片分布页。
- **Backend**: 采集器改造为「按需激活的共享采集」—— 由「启动即无条件定时全量采集」改为按 `(集群, kind)` 维护共享缓存：单飞去重保证并发请求只打一次 ES，读操作刷新 `lastAccess`、空闲超过 60s 即淘汰并停止采集；支持 stale-while-revalidate（过期先返回旧数据再后台刷新，仅冷启动同步等待）。`ListClusters` 改用 `StatusSnapshot`，未新增 kind。
- **Backend**: 补齐数据底座 —— 新增 `nodes` / `indices` / `shards` 三个采集 kind（`_cat/nodes`、`_cat/indices`、`_cat/shards`），并新增只读接口 `GET /api/clusters/:id/{nodes,indices,shards}`，统一返回 `{ data, updated_at }`；新增超时更宽松的 `CatalogClient`（30s）承载 `_cat` 列表拉取。
- **Backend**: 新增集群总览聚合接口 `GET /api/clusters/:id/overview` —— 聚合 status / nodes / indices / shards，产出健康条、关键计数、节点表（含每节点承载分片数）与问题清单（未分配分片 / 迁移中分片 / 非 Green 索引，各取前 20 条并附总数）；`?refresh=1` 可强制重采。`_cluster/health` 采集字段扩展至 active/relocating/initializing 分片与 active_shards_percent；`_cat/indices` 改用 `bytes=b` 以便汇总存储量。
- **Frontend**: 实现集群总览页 `ClusterOverview.vue` —— 健康条（状态 + active shards 百分比 + 未分配/迁移/初始化）、6 项关键计数、节点表（heap/cpu/disk 超阈值着色、分片数列）、问题清单（三张表格，全清时显示「一切正常」）；页面 15s 轮询作为采集心跳，支持手动强制刷新，并展示数据新鲜度。
- **Frontend**: 实现分片分布页 `ClusterShards.vue` —— 节点 × 索引分片矩阵，支持「按节点看 / 按索引看」视角切换、索引名搜索、「只看有问题的索引」过滤与索引轴分页；分片方块用颜色表示状态、实心/空心区分主副分片（形状+颜色双编码），未分配分片归入「(未分配)」伪节点；侧边栏新增 Shards 入口。
- **Backend**: `GET /api/clusters/:id/{nodes,indices,shards}` 支持 `?refresh=1` 强制重采（新增 `es.RefreshKind`）。
- **Backend**: `GET /api/clusters/:id/indices` 改为后端过滤 + 分页 —— 新增 `search` / `health` / `status` 过滤与 `page` / `page_size` 分页（默认 20、上限 200），返回 `{ data, total, page, page_size, updated_at }`；过滤分页基于采集缓存切片，不额外请求 ES。
- **Frontend**: 实现索引列表页 `ClusterIndices.vue` —— 表格（索引 / health / 状态 / 主副 / 文档 / 存储）、索引名搜索（300ms 防抖）、health 与 status 过滤、排序（索引名 / 存储 / 文档 / health，可切换升序降序）、分页器与每页条数切换；15s 轮询心跳、手动强制刷新、新鲜度展示；侧边栏新增 Indices 入口。
- **Backend**: `GET /api/clusters/:id/indices` 增加 `sort` / `order` 参数（索引名 / 存储 / 文档 / health），排序在过滤之后、分页之前进行；排序时复制切片，避免就地修改被并发读取的共享采集缓存。
- **Docs**: 新增设计原则 §2.4「信息密度优先，少栏目」（优先并入既有页面，不默认新增导航项，不做 Kibana 式功能广度）；新增 §6.4「页面职责边界」（Overview 分诊 / Indices 清单容量 / Shards 分布，明确不重合）；§11 边界拓展 —— 不做完整监控体系（时序存储 / 趋势 / 告警 / APM / 日志），但允许在既有页面内就地计算展示轻量运维信号。

### 2026-10-01
- **Backend**: 新增后台采集器 `internal/es/collector.go` —— 30s 一轮、并发采集、内存快照；进程启动即采一轮；失败标记 `unreachable` 并保留失败原因，不与 `red` 混淆。
- **Backend**: `GET /api/clusters` 现在附带采集快照 `status`（reachable / health / version / node_count / index_count / unassigned_shards / collected_at）。
- **Backend**: 新增 `PUT /api/clusters/:id`（此前缺失）与 `POST /api/clusters/refresh`（手动触发重采）。
- **Backend**: 落实 PRD §5「先探测、后保存」—— 新增集群、以及编辑时连接信息（hosts/认证）发生变更，都必须探测通过才落库；仅改名称/颜色/备注不会因集群临时不可达而被卡住。编辑时密码/API Key 留空表示不修改。
- **Frontend**: 新增 Cluster Hub 作为首屏（`/`）—— 卡片墙承载连接管理（增删改）与状态展示，含空状态引导、探测预览弹窗、删除二次确认、手动刷新；默认按 `unreachable → red → yellow → green` 排序，同级按未分配分片降序。
- **Frontend**: 移除顶部集群选择器，活动集群改由 URL 决定（`/cluster/:id/overview`、`/cluster/:id/console`）；Dev Console 迁入集群路由，元数据 store 改为显式传入 clusterId。
- **Frontend**: 新增集群总览占位页，作为后续 Cerebro 核心页面（节点 / 索引 / 分片）的入口。
- **Fix**: 修复 `pnpm build` 因 7 个未使用变量导致的 TypeScript 检查失败。

### 2026-09-29
- **Docs**: PRD 升级至 v0.3，产品定位收缩为「Cerebro 替代品」，确立「功能先行、UI 后置」原则，Dev Console 推迟至 Phase 2；差异化押注等 6 项未决问题显式列为待定。
- **Docs**: TASKS 看板对齐 Phase 1 与真实进度。
- **Refactor**: 抽出 `internal/es` 请求层（`Target` / `NewRequest` / 共享 HTTP 客户端），`/api/proxy` 改为复用该层，消除 URL 拼接与认证逻辑的重复。
- **Backend**: 新增 `Probe` 探测能力（`GET /` + `GET /_cluster/health`）与 `POST /api/clusters/probe`，实现「先探测、后保存」，避免写入连不上的集群。
- **Frontend**: 添加集群弹窗新增 "Test Connection"，保存前展示探测结果（集群名 / 版本 / health / 节点数）。

### 2026-04-02
- **Frontend**: 引入 Pinia 和 VueUse 进行全局状态管理，实现 `useClusterStore` 统一管理集群列表与选中状态。
- **Frontend**: 实现集群选中 ID 的 LocalStorage 持久化，确保刷新页面后状态不丢失。
- **Frontend**: 完成 `MainLayout` 集群选择逻辑重构，对接后端真实 API，实现集群添加与切换闭环。
- **Dev**: 配置 Vite 代理 (`/api` -> `localhost:18080`)，解决开发环境下跨域与端口不一致问题。

### 2026-03-27
- **Frontend**: 集成 UnoCSS 并配置原型主题色，完成 `MainLayout`, `Dashboard`, `DevConsole` 组件化迁移。
- **Frontend**: 配置 Vue Router 实现多页面切换，切换包管理器为 pnpm 并优化 `.npmrc` 配置。
- **Proxy**: 实现 `/api/proxy` 通用转发，支持通过 `X-Cluster-ID` 自动路由至对应集群并处理 Auth (Basic/ApiKey)。

### 2026-03-26
- **API**: 实现集群管理的 CRUD 接口 (List/Create/Get/Delete)，支持集群信息的增删改查。
- **Models**: 重构 `Cluster` 模型，通过 `StringArray` 类型支持 SQLite JSON 序列化存储 `hosts` 列表。
- **Router**: 提取路由逻辑至独立 `router` 模块，优化 `main.go` 入口。
- **Docs**: 完成 `PRD.md` 和 `Architecture.md` 的初步编写，明确产品定义与技术架构。
- **Docs**: 初始化 `TASKS.md` 和 `CHANGELOG.md` 任务追踪体系。
- **Setup**: 按照架构文档搭建后端 Go 目录结构及前端 Vue 3 项目骨架。
- **Setup**: 配置 Air 实现 Go 热重载，并完善 Git 忽略文件及目录保护。
- **Database**: 初始化 SQLite 数据库连接及核心表结构迁移逻辑。
