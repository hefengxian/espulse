# TASKS — ESPulse 任务看板

> 状态：进行中 | 当前阶段：**Phase 2 推进（统一工作台已转正）+ Phase 4 macOS 桌面版启动**
> 原则：功能先行，UI 后置（见 [PRD.md](./PRD.md) §2.1）；单页看板、信息密集、动态刷新、不增栏目（见 §2.4）；**集群只有一个落地页 `/workbench/:id`**（见 §2.5）

---

## 🎯 当前目标

把「统一工作台」做成唯一入口并补齐缺口，然后进入 Dev Console 的智能提示；并行启动 macOS 桌面版打包（见 Phase 4），先出可用版本。

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

- [x] **等待期的空窗与数据归属**：切集群即清空上一个集群的数据并作废在途请求（迟到的响应一律丢弃），等待期显示骨架屏，「暂无索引」与「过滤无匹配」分开提示；数据未到时指标条与活动区降级为占位
- [ ] **别名视图迁入工作台**：`_cat/aliases` 采集与 `GET /api/clusters/:id/aliases` 接口都在，UI 视图（别名 → 索引映射、写别名、filter / routing）随旧版索引页一起删掉了，需要在工作台里重建入口
- [ ] **索引视角看不到迁移中的分片**：relocating 分片在活动区能列出来，但索引视角的网格里没有对应的方块（Cerebro 的索引视图能看出分片正在迁移，参考它的呈现）。要先把 `_cat/shards` 迁移行的语义弄准 —— `state=RELOCATING` 时 `node` 指向哪一端、目标节点从哪儿取 —— 再决定索引视角怎么画：迁移中的分片在源 / 目标两列各标一处，或单列一个「迁移中」格；同时保证指标条的「迁移」图例计数、方块配色与 tooltip 都与之对得上
- [ ] **视角 / 过滤条件 URL 化**：当前视角、搜索、「隐藏 `.` 开头」「只看有问题的」「按前缀归并」、排序都在组件内部状态，刷新即丢、深链不可分享（PRD §2.5 规则）
- [x] **节点可见性对齐 Cerebro**：索引视角只保留 data 节点（`node.role` 含 `d/h/w/c/f/s` 任一角色码，不看分片数），节点视角保持展示全部节点；纯前端由 `overview.nodes` 派生，不改后端（PRD §6.5）
- [ ] 键盘支持（`⌘K` / 工作台-控制台切换 / `/` 聚焦）：交互习惯稳定后再做，骨架预留位置（PRD §2.5）
- [ ] **索引元信息下沉到底部状态栏（悬浮查看）**：网格索引行只留名字 + 健康点（现有行内定宽微列改为次要态或可收起），鼠标悬浮某行时，把该索引的完整信息（状态 / 主副分片 / 文档数 / 存储，必要时补速率等明细）显示在底部状态栏上，随行切换。好处是行内零占用、密度最高，且不会像换行方案那样撑高行高；代价是信息不常驻、无法跨行对比。落地注意：一是别用「悬浮撑高行高」的做法（会引起下方行位移抖动），要走固定高度的状态栏；二是状态栏里各字段固定位宽，避免切行时文字左右跳动

### 3. 技术债

- [ ] **`TestIndexRingSurvivesProcessRestart` 失败**：模拟进程重启后重建样本环，无法用落库样本继续差分（实际 `window=0s`，期望 `10s`）—— 先定位 `loadIndexSamples` 的窗口判定（`internal/es/rate.go` / `snapshot_store.go`），这是「速率列不掉档」的核心收益，不宜带红灯进 Phase 2

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

- [x] **分片恢复进度（`_recovery`）**：新增采集 kind `recovery`（`_recovery?active_only=true`，5s，共享缓存 / 单飞去重 / 退避 / 落库与其他 kind 同机制）与 `GET /api/clusters/:id/recovery`，响应拍平成「一行一个分片副本」。指标条的「恢复吞吐 / 剩余」由留白改为真实值（吞吐 = Σ分片平均速率，剩余 = 按该吞吐的 ETA）；活动区恢复表由 5 列扩到 8 列 —— 分片复用网格那套方块（实心 = 主 / 描边 = 副），加阶段 / 源 → 目标 / 文件 · 字节 · translog 进度 / 耗时；恢复类型不占列（移入方块 title），表按耗时降序、不截断行数，进度条的绝对值放 title；计数与列表同源（不再用 `_cat/shards` 另拼一份）。不做「卡住」阈值判定、不做完成留痕 —— 该接口只回答「此刻」
- [x] **指标条补集群版本**：`overview` 响应增加 `version`，叠在健康胶囊正上方并居中 —— 版本此前只在 Hub 卡片上出现，工作台看不到；不用集群列表里那份 `status.version`，因为它只在挂载时拉一次、不跟随轮询。
- [ ] 配置文件预置集群
- [ ] 集群凭据 AES-GCM 加密存储
- [ ] 多实例部署下的跨进程采集去重（见 PRD §10）

### Phase 4 — macOS 桌面版打包（本轮）

> 决策（2026-10）：**只做 macOS**，目标是尽快出可用版本；不做服务器版，Windows / Linux 留到后续。原则：**能不自己实现就不自己实现** —— 用系统默认菜单、不做托盘、不做自定义标题栏、不做窗口状态记忆。
> 构建产物跟随当前机器架构（两台 Mac 各自出包即可）；暂不做 universal 双架构包。
> 顺序固定：先让 Go 单二进制自己托管前端（浏览器可验收），再套 Electron 壳，最后打包。

#### P0 后端：单二进制托管前端（Electron 的硬前置）

- [ ] `frontend/vite.config.ts`：`base: '/'`、`build.outDir: '../internal/web/dist'`、`emptyOutDir: true`
- [ ] 新增 `internal/web`：`//go:embed all:dist` + 静态 handler —— `assets/*` 长缓存、`index.html` 不缓存，非 `/api` 未命中回 `index.html`
- [ ] 保留 `internal/web/dist/.gitkeep` 占位，并在根 `.gitignore` 放行，保证前端未构建时 `go run .` 仍可编译
- [ ] `internal/router/router.go`：API 注册完后挂载 `internal/web` 的 handler
- [ ] `main.go` 参数化（`flag` 即可，不引配置库）：`--host`（默认 `127.0.0.1`）、`--port`（默认 `18080`，`0` = 内核分配）、`--data-dir`（默认 `./data`）、`--log-file`
- [ ] 启动握手：监听成功后 stdout 打印 `ESPULSE_READY url=http://<host>:<port>`；补 `GET /api/health`
- [ ] 日志：指定 `--log-file` 时同时写文件（桌面端传 userData 路径，无控制台也能排障）；Gin 用 release mode
- [ ] 退出：`SIGINT / SIGTERM` → 停采集器、关 DB

验收：`go run . --data-dir /tmp/espulse-test`，浏览器打开 `http://127.0.0.1:18080` 全功能可用（无需 Vite）；`curl -s 127.0.0.1:18080/api/health` 返回正常，且 stdout 有握手行。

#### P1 Electron 壳（最小实现）

- [ ] 新增 `desktop/`：`package.json`（`main: main.js`，devDeps：`electron` / `electron-builder`）、`main.js`
- [ ] 启动链路：单实例锁 → `spawn` 后端（`--host 127.0.0.1 --port 0 --data-dir <userData>/data --log-file <userData>/logs/espulse.log`）→ 读 stdout 的 `ESPULSE_READY`（10s 超时弹错误框）→ `loadURL`
- [ ] 窗口：1440×900、最小 1024×700，不记忆尺寸 / 位置
- [ ] 菜单：**用 Electron 默认菜单**（保留 Cmd+C/V 等编辑快捷键），不自定义、不隐藏
- [ ] 安全：`contextIsolation: true`、`nodeIntegration: false`、`sandbox: true`、无 preload；`setWindowOpenHandler` → 系统浏览器；`will-navigate` 拦截站外地址
- [ ] 生命周期：关窗即退出；`before-quit` 结束后端；后端非 0 退出提示「重启 / 退出」；macOS `activate` 重开窗口；生产不开 DevTools
- [ ] 开发模式：`ESPULSE_DEV_URL=http://localhost:5173` 时加载 Vite dev server

验收：`pnpm dev`（frontend）+ `go run .` + `npx electron desktop` 能正常使用；退出后 `pgrep -f espulse` 无残留。

#### P2 打包（macOS）

- [ ] 图标：1024px PNG → `iconutil` 生成 `desktop/build/icon.icns`（从 `frontend/public/` 现有 logo 出图）
- [ ] electron-builder 配置（`desktop/package.json` 内）：`appId: com.hefengxian.espulse`、`productName: ESPulse`、`asar: true`、`mac.target: [dmg, zip]`、`mac.category: public.app-category.developer-tools`、`mac.identity: null`（本阶段不签名）、Go 二进制走 `extraResources`（asar 外）
- [ ] `scripts/build-macos.sh`：前端 build → `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o desktop/backend/espulse .` → `cd desktop && npx electron-builder --mac`
- [ ] 兜底：`afterPack` 给 `backend/espulse` 补可执行位
- [ ] `.gitignore`：`desktop/node_modules/`、`desktop/backend/`、`desktop/release/`
- [ ] README 写明未签名版本的打开方式：右键「打开」，或 `xattr -dr com.apple.quarantine /Applications/ESPulse.app`

验收：dmg 安装后能启动、能连 ES；数据落在 `~/Library/Application Support/ESPulse/`；关窗后 `pgrep -f espulse` 无残留。

#### P3 文档同步

- [ ] ARCHITECTURE（§2 目录、§6 内嵌路径、§7 构建、§8 Electron）、TASKS 勾选、CHANGELOG

#### 本阶段明确不做

Windows / Linux 打包、服务器模式、托盘、自定义菜单与标题栏、窗口状态记忆、自动更新、代码签名与公证、universal 双架构包。

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
- [x] 副本告警：索引网格 `主/副` 列对副本 0 标黄（副本数字加虚线底线 + `title` 说明），前缀归并的组头按组内无副本索引数单独标记
- [x] 索引网格指标列：主/副、文档、存储由索引名右侧的元信息微列改为独立表格列，列头可按文档 / 存储排序；组头在同一列展示汇总值
- [x] 节点资源用量条：节点视角 Heap / CPU / 磁盘改为「百分比 + 底部用量轨」，绿 / 黄 / 红按原阈值分级，不增高行高
- [x] 数据接口 `GET /api/clusters/:id/overview`（聚合 status / nodes / indices / shards，支持 `?refresh=1`）
- [x] 快照未就绪时的空态 / 不可达态 / 15s 轮询与顶栏「更新于 N 秒前」

### 页面骨架与交互

- [x] 取消侧边栏；顶栏承载「集群切换器 │ 集群级刷新 │ 更新于 N 秒前 │ 语言 / 主题 │ 工作台·控制台模式切换」
- [x] 集群切换器：点开即选、最近使用优先、集群 > 8 个时提供搜索、底部「管理集群 →」
- [x] 切集群保持当前页面（push 历史，后退可回上一个集群）
- [x] Hub 退回「管理 + 浏览」：卡片墙 + 增删改 + 共用顶栏
- [x] 色板单一真相源：工作台的 `--bg/--text/--green/...` 全部指向 `--esp-*`，Hub 与工作台共用一套配色
- [x] 顶栏统一 44px；Hub 的 Naive UI 控件统一取 small 档（表单尺寸由 `n-form` 下发，控件不单独指定）
- [x] 刷新收敛为集群级单入口；无效集群 id 统一收敛回 Hub
- [x] 清理原型残留（`Dashboard.vue` / `HelloWorld.vue` 及未引用 asset）
- [x] 主题抽取为 `useTheme` 组合式（持久化到 localStorage）+ 接入 vue-i18n 与语言切换
- [x] ~~模块 keep-alive / 过滤条件按集群共享~~ 随模块页一并取消（工作台自带同屏两态不销毁）

### 项目地基

- [x] 文档：PRD / ARCHITECTURE / TASKS / CHANGELOG 体系
- [x] 后端：SQLite 初始化与建表迁移
- [x] 工程：Air 热重载、.gitignore、.gitkeep、pnpm + Vite 代理、UnoCSS 主题
