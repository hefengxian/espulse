# TASKS — ESPulse 任务看板

> 状态：进行中 | 当前阶段：**Phase 1 收口 + Phase 2 起步（统一工作台已转正）**
> 原则：功能先行，UI 后置（见 [PRD.md](./PRD.md) §2.1）；单页看板、信息密集、动态刷新、不增栏目（见 §2.4）；**集群只有一个落地页 `/workbench/:id`**（见 §2.5）

---

## 🎯 当前目标

把「统一工作台」做成唯一入口并补齐缺口，然后进入 Dev Console 的智能提示。

---

## 🛠 正在进行 (In Progress)

### 1. 工作台转正（本轮）

> `/workbench/:id` 从「与旧版并存的 v5 原型」转为**集群的唯一落地页**。

- [x] 路由转正：`/workbench/:id` 为集群落地页；旧路径 `/cluster/:id/{overview,indices,shards,console}` 重定向到对应集群的工作台（书签 / 历史兼容，不做兼容页）
- [x] 删除旧版模块页：`ClusterOverview.vue` / `ClusterIndices.vue` / `ClusterShards.vue` / `DevConsole.vue`（功能已并入工作台）
- [x] 删除只服务旧页面的 `composables/useViewFilters.ts`（旧版「按集群记忆过滤条件」的载体）
- [x] 顶栏去掉模块 Tab：模块之间不再有跳转，只有工作台内的视角 / 模式切换
- [x] Cluster Hub 整合：集群卡片点击直接进工作台（不再跳旧版 Overview）
- [x] 集群切换器统一目标：Hub 或任意工作台内切换，落地都是 `/workbench/:id`
- [x] 无效集群 id 的守卫移入工作台（原先在包裹旧模块页的 `MainLayout` 里，删集群后的书签不会停在空工作台）
- [x] 命名收尾：去掉并存时期的 `v5` 后缀（`views/Workbench.vue`、`styles/workbench.less`、路由名 `Workbench`；原型稿 `ui_prototype/espulse-workbench-v5.html` 是设计参考，保留原名）
- [x] 文档转正：PRD §2.4 / §2.5 / §3 / §6 / §7、ARCHITECTURE §1 §2 §4、CHANGELOG

### 2. 工作台的已知缺口

- [ ] **别名视图迁入工作台**：`_cat/aliases` 采集与 `GET /api/clusters/:id/aliases` 接口都在，UI 视图（别名 → 索引映射、写别名、filter / routing）随旧版索引页一起删掉了，需要在工作台里重建入口
- [ ] **视角 / 过滤条件 URL 化**：当前视角、搜索、「隐藏 `.` 开头」「只看有问题的」「按前缀归并」、排序都在组件内部状态，刷新即丢、深链不可分享（PRD §2.5 规则）
- [ ] 键盘支持（`⌘K` / 工作台-控制台切换 / `/` 聚焦）：交互习惯稳定后再做，骨架预留位置（PRD §2.5）

### 3. 技术债

- [ ] **`TestIndexRingSurvivesProcessRestart` 失败**：模拟进程重启后重建样本环，无法用落库样本继续差分（实际 `window=0s`，期望 `10s`）—— 先定位 `loadIndexSamples` 的窗口判定（`internal/es/rate.go` / `snapshot_store.go`），这是「速率列不掉档」的核心收益，不宜带红灯进 Phase 2
- [ ] 顺手：`components/workbench/WorkbenchGrid.vue` 里未使用的 `fmtBytes` / `fmtInt` 导入会让 `pnpm build` 的 `vue-tsc` 步骤失败

---

## ⏭ 后续

### Phase 2 — Dev Console（下一步重点）

- [x] Monaco Editor 集成与真实请求发送
- [x] 命令目录侧边栏：解析当前编辑器命令行生成大纲，点击定位到对应行
- [x] 编辑内容按集群保持：草稿落 localStorage，切集群各自独立、刷新/重开恢复
- [x] 工作台内以顶栏模式切换承载控制台（首次切入才挂载，之后保活）
- [ ] **智能提示（重点增强）**：上下文感知的补全 —— 按命令行 / HTTP 方法 / 路径 / 请求体位置，推断 ES API、索引 / 别名、字段、查询 DSL 片段与常用参数（详见 PRD §6 Phase 2）
- [ ] ~~请求历史（最近 200 条）~~ 暂缓，后续再评估
- [ ] ~~命令收藏夹与变量占位符~~ 暂缓，后续再评估

### Phase 3 — 差异化探索（待定）

- [ ] 差异化押注确定（见 PRD §10）
- [ ] 多集群统一视角
- [ ] 跨集群执行对比

### 平台能力

- [ ] `_cat/recovery` 接入：补上指标条的「恢复吞吐 / 剩余量」（当前留白）
- [ ] 配置文件预置集群
- [ ] 集群凭据 AES-GCM 加密存储
- [ ] 多实例部署下的跨进程采集去重（见 PRD §10）

### 最后环节 — 打包与多平台发布

> **前置条件：直到我们认为所有设计功能都完成之前，前面各阶段都不考虑实现打包与发布。**

- [ ] `embed.FS` 内嵌前端 + 单二进制构建脚本
- [ ] 交叉编译 / 多平台发布（Windows / macOS / Linux）
- [ ] Electron 打包与托盘（Windows / macOS / Linux）

---

## ✅ 已完成（Phase 1 归档）

### 集群连接管理（含连通性探测）

- [x] 集群 CRUD（List / Create / Update / Get / Delete）
- [x] 可复用的 ES 请求层 `internal/es/client.go`
- [x] `Probe` 探测（`GET /` + `GET /_cluster/health`）与 `POST /api/clusters/probe`（不落库）
- [x] 强制「先探测、后保存」（新增、以及编辑时连接信息变更）
- [x] 多 host 故障转移（仅传输层错误才切换；记住最近可用节点优先命中；全失败报「N 个 host 均不可达」）
- [x] Hub 卡片墙（空状态 / 新增 / 编辑 / 删除二次确认 / 手动刷新）；活动集群由 URL 决定

### 集群健康采集

- [x] `internal/es/collector.go`：按需激活的共享采集（5s 间隔 + 失败指数退避）+ 内存快照
- [x] `(集群, kind)` 共享缓存 + 单飞去重 + `lastAccess` 空闲淘汰（PRD §6.3）
- [x] kind 扩展：`nodes` / `shards` / `indices` / `aliases`
- [x] 数据接口 `GET /api/clusters/:id/{nodes,indices,shards,aliases}`，统一返回 `{ data, updated_at }`
- [x] 快照落库（`snapshots` 表，gzip）：内存淘汰 / 进程重启后先渲染落库快照再后台刷新
- [x] 速率样本环（`index_samples` 表）与索引写入 / 搜索速率（就地差分，窗口 60s，随快照返回 `rate_window_ms`）

### 统一工作台

- [x] 指标条：health + 索引健康环 + active 占比；节点 / 索引 / 主副分片 / 文档 / 存储计数；分片构成堆叠条
- [x] 活动区：relocating / initializing 分片、未分配分片（含原因）、非 green 索引；「健康 / 恢复」折叠开关；点索引名就地过滤到网格
- [x] 统一网格：索引视角（行 = 索引、列 = 节点）与节点视角（行 = 节点、列 = 索引）一键切换；分片方块按状态着色、实心 / 虚线区分主副；未分配分片单列
- [x] 网格过滤与排序：索引名搜索、隐藏 `.` 开头索引（默认开）、只看有问题的索引、按前缀归并（组头汇总）、排序（写入 / 查询速率、文档、存储、health）、分页
- [x] 数据接口 `GET /api/clusters/:id/overview`（聚合 status / nodes / indices / shards，支持 `?refresh=1`）
- [x] 快照未就绪时的空态 / 不可达态 / 15s 轮询与顶栏「更新于 N 秒前」

### 页面骨架与交互

- [x] 取消侧边栏；顶栏承载「集群切换器 │ 集群级刷新 │ 更新于 N 秒前 │ 语言 / 主题 │ 工作台·控制台模式切换」
- [x] 集群切换器：点开即选、最近使用优先、集群 > 8 个时提供搜索、底部「管理集群 →」
- [x] 切集群保持当前页面（push 历史，后退可回上一个集群）
- [x] Hub 退回「管理 + 浏览」：卡片墙 + 增删改 + 共用顶栏
- [x] 刷新收敛为集群级单入口；无效集群 id 统一收敛回 Hub
- [x] 清理原型残留（`Dashboard.vue` / `HelloWorld.vue` 及未引用 asset）
- [x] 主题抽取为 `useTheme` 组合式（持久化到 localStorage）+ 接入 vue-i18n 与语言切换
- [x] ~~模块 keep-alive / 过滤条件按集群共享~~ 随模块页一并取消（工作台自带同屏两态不销毁）

### 项目地基

- [x] 文档：PRD / ARCHITECTURE / TASKS / CHANGELOG 体系
- [x] 后端：SQLite 初始化与建表迁移
- [x] 工程：Air 热重载、.gitignore、.gitkeep、pnpm + Vite 代理、UnoCSS 主题
