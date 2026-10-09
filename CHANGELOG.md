# CHANGELOG — ESPulse 变更日志

> 记录项目的每一个足迹。

---

## [Unreleased]

### 2026-10-09

#### 字体与排版
- **Frontend**: 正文字体换成 **Inter Variable**，等宽换成 **JetBrains Mono Variable**（`@fontsource-variable/*`，替换 Instrument Sans / Geist Mono）—— **变量字体一个文件覆盖 100–900**。此前只引入了 Geist Mono 400，而代码里大量使用 `b` / `font-600` / `font-700`（汇总行、统计卡、Dev Console 方法徽章），浏览器只能用合成粗体，字重发糊；也不再有「某个字重忘了引」的坑。实测 `.font-mono b` 从合成 700 变为真实字重。
- **Frontend**: 根元素加 `font-variant-numeric: tabular-nums` —— 等宽数位让表格数字变化时列宽不跳（实测 Inter 数字串 `1111111111` 与 `8888888888` 在默认比例数位下相差 60%，开启后完全一致）。等宽字体自带该特性，不受影响。
- **Frontend**: **显式声明中文回退栈**（`PingFang SC` → `Hiragino Sans GB` → `微软雅黑` → `Noto Sans SC` / `思源黑体` → `system-ui`）。字母类字体无 CJK 字形，中文本来就落在系统字体上（实测标题 100% 由 PingFang SC 渲染）；列出回退栈是为了让中英混排的落点可预期，不再掉到某个不可控的默认字形。**CJK 不内嵌**（体积考虑，见下）。
- **Frontend**: Dev Console 的 Monaco `fontFamily` 由 `var(--esp-font-mono)` 改为**解析后的实际字体栈** —— Monaco 要在 canvas 里量字符宽度，解析不了 `var()`，之前一直静默回退到 Monaco 默认字体（`-apple-system / Segoe WPC`）；现在 `--esp-font-mono` 取值后传入，编辑器用上 JetBrains Mono。
- 体积：latin 子集 woff2 共 89KB（Inter 48KB + JetBrains Mono 40KB），构建产物里 12 个面合计 312KB；构建期只 emit 一次，运行期由 `unicode-range` 决定只下载 latin。`frontend/ui_prototype/*.html` 是独立原型页（Google Fonts 引入、未被引用），本轮未同步。

#### 分片方块与表格边框
- **Frontend**: 修复**全站 `border-*` 工具类静默失效** —— UnoCSS 未引入 reset，浏览器默认 `border-style: none`，只设置 `border-width` / `border-color` 画不出任何线（只有 `border-dashed` 这类自带 `border-style` 的写法侥幸可见）。这正是「分片页竖向没有分割线」与「副本分片只剩一个数字」的同一个根因：卡片边框、表格网格线、分片方块全部没画出来。`uno.config.ts` preflight 补 `*, ::before, ::after { border-style: solid; border-width: 0 }`；**刻意不设默认 `border-color`**，保留 currentColor，避免破坏既有「border + 动态文字色」的写法。
- **Frontend**: 副本分片方块改为**虚线框** —— 实心 = 主分片、虚线框 = 副本分片，与 Cerebro 观感一致，形状 + 颜色双编码不变；工具栏图例改为与 `CHIP_STYLE` 同源，避免两处各写一份而漂移。
- **Frontend**: 分片矩阵 `border-collapse` 改 **`border-separate` + `border-spacing: 0`** —— collapse 模式下 sticky 表头 / 首列的边框会跟着内容一起滚走；单元格改为**固定两格宽**（`w-11.5` + `grid-cols-2`），方块在所有列里对齐成统一网格，不再随内容宽度参差；行悬浮时首个 sticky 列同步高亮。
- **Docs**: PRD §6.2 副本分片的描述由「空心」更正为「虚线框」。

#### 页面骨架与交互层级（PRD §2.5）
- **Frontend**: **取消侧边栏**，导航收敛为顶栏两层 —— 左侧**集群切换器**（我在哪个集群）、右侧**模块 Tab**（Overview / Indices / Shards / Dev Console，看哪个维度），中间用分隔符明确层级。原侧边栏把「回集群列表」与「集群内四个视图」平铺成同一组、并要求菜单形态随上下文变形（Hub 页只剩 1 项），层级本身就是错的；取消后固定宽度还给数据表，`isCollapsed` 状态一并消失。
- **Frontend**: 新增 `components/ClusterSwitcher.vue` —— 点开即选，不再经 Hub 中转（切集群由 3 跳降为 1 跳）；选中后**保持当前模块**（`/cluster/A/shards` → `/cluster/B/shards`），用 push 写历史以便后退回上一个集群；列表**按最近使用优先**排序（覆盖收藏 / pin 的主要收益），集群数 > 8 时显示搜索框并自动聚焦；底部固定「管理集群 →」回 Hub。Hub 页显示「全部集群」。**替换了原先 header 上那个点击无效的集群胶囊**。
- **Frontend**: **过滤 / 排序 / 分页挂在「集群」上，不挂在「模块」上**（新增 `composables/useViewFilters.ts`）—— 同一集群内各模块共享同一份条件（在索引页筛出的索引范围，切到分片页依然生效），跨集群互相隔离。状态解析优先级为 **URL query → 该集群上次的状态（localStorage）→ 默认值**：URL 有参数就以 URL 为准（可分享、可后退），否则回落到该集群上次的条件。与默认值相同的项不写进地址栏，切换集群时先清掉地址栏里属于本模块的参数，避免把上一个集群的条件带过去。
- **Frontend**: **刷新收敛为全局单入口**（新增 `composables/useGlobalRefresh.ts`）—— 顶栏的刷新作用于「当前集群的全部数据」而非当前模块，由当前激活的模块注册实现；「更新于 N 秒前」常驻顶栏（超过三个采集周期时提示「正在刷新…」）。各页面自带的刷新按钮与本地计时器全部移除。理由：后端采集本就是 `(集群, kind)` 共享的，按模块各自刷新只是浪费，而且「在索引页点刷新、总览页没变」会让人不敢信数据。Hub 页的刷新则作用于集群列表本身。
- **Frontend**: 模块 **keep-alive**，切走再回来不丢视图状态（Console 尤其需要）。随之把三个页面的**轮询 / 计时器从 `onMounted` / `onBeforeUnmount` 迁到 `onActivated` / `onDeactivated`** —— 否则被保活的页面会在后台一直轮询，既浪费请求，也让「轮询即活跃心跳」的语义失效（见 PRD §6.3）。
- **Frontend**: 总览页问题清单中的索引名变成**深链**，点击直接跳到分片分布并带上过滤条件（`?search=<索引>&onlyProblem=1`），不再需要用户自己切页面再筛一遍。这是唯一保留的模块间跳转 —— 它不是「下钻」，而是换视角看同一批行。
- **Frontend**: **无效集群 id 统一收敛** —— 集群列表加载成功后若 URL 中的集群不存在（已删除 / 旧书签），重定向回 Hub，不再渲染一套点进去为空的模块入口。`stores/cluster.ts` 新增 `loaded` 标记，用来区分「还没加载完」与「确实不存在」。
- **Frontend**: 主题与折叠状态不再是一次性副作用 —— 主题选择持久化到 localStorage，刷新后保持。
- **Frontend**: 清理原型残留文件：`views/Dashboard.vue`（157 行假数据「总览页」，与真实总览页重名易混淆）、`components/HelloWorld.vue` 及三个未引用的 asset（`hero.png` / `vite.svg` / `vue.svg`）。这些文件不在路由与任何 import 中。
- **Docs**: PRD 升级至 v0.4 —— 新增 **§2.5 页面骨架与交互层级**（四层结构、切集群/模块规则、状态作用域、刷新语义、keep-alive 约束，以及本轮显式不做的键盘与收藏）；§2.4「不新增栏目」补充说明顶栏模块 Tab 属于页面框架而非栏目；§4 补充「Hub 不是必经路径」。TASKS 新增「页面骨架与交互」小节。

- **Backend**: 采集快照**落库**，解决「很久没用过的集群，进总览页要等第一次刷新」——此前快照只存在于单进程内存里，空闲超过 60s 被淘汰、或进程重启后就彻底消失，读取只能同步等一次采集（最坏 `host 数 × (5s + 30s)`）。现在每轮采集成功后立即写入 SQLite `snapshots` 表（gzip 压缩），读取端在内存无数据时先取回落库快照立即返回、同时后台刷新；**只有「从未采集过」的集群（首次添加）才需要同步等一次**。落库快照以 gzip 魔数判断区分，兼容未压缩的历史行，无需 schema 迁移；集群不可达也照常落库，重启后先显示「不可达（采集于 X）」再被纠正；删除集群时同步清理。新增 `internal/es/snapshot_store.go`。
- **Backend**: 快照落库做 **gzip(BestSpeed) 压缩** —— 采集间隔 5s，而 `_cat/shards` / `_cat/indices` 的 JSON 很大（实测 12657 索引的集群分别为 6.6MB / 3.2MB），不压缩等于约 2MB/s 的持续写盘；实测压缩比 16.5x / 9.4x，写盘降到约 150KB/s。同时为 SQLite 开启 `journal_mode=WAL` + `busy_timeout=5000` + `synchronous=NORMAL`，避免高频写入与读取互相阻塞（`internal/database/db.go`）。
- **Backend**: 新增索引**写入 / 搜索速率**（`internal/es/rate.go`）—— 复用 `_cat/indices` 自带的累计计数列 `indexing.index_total` / `search.query_total`（6.x 起即有），不额外调 `_stats`。采集器按集群维护 `index_samples` 样本环（保留期 = 窗口 60s，约 12 个样本），速率 = 窗口内最早样本与本轮样本的计数差 / 实际时间差。样本同样落库，因此 air 热重载这类短时间重启后样本环是「温」的、速率列不断档。计数倒退（索引被删除重建）视为无效样本并清空该索引的环，绝不显示负数；样本不足时留空，前端显示「—」。`kindSpec` 新增 `decode` / `decorate` 钩子，派生值在采集线程内补齐后再随快照落库。
- **Backend**: 索引列表接口新增 `rate_window_ms` 与实际窗口，`sort` 支持 `index_rate` / `search_rate`；速率缺失的行在排序中恒排最后，不随升降序翻转（避免降序看热点时最前面是一片「—」）。`GetIndices` 返回值由 `[]Index` 改为带窗口信息的 `IndexList`。
- **Frontend**: 索引页新增「写入/s」「查询/s」两列（零值显示 `0`、无样本显示 `—`），列头悬浮提示说明「速率 = 计数差 / 窗口长度」及当前窗口秒数；排序下拉新增「按写入速率」「按查询速率」。
- **Frontend**: 抽出 `frontend/src/utils/freshness.ts`，把散落在 Cluster Hub / 总览 / 索引 / 分片四处的新鲜度文案统一为「秒 / 分钟 / 小时 / 天」——此前长间隔会渲染成「更新于 7200 秒前」，读不通也掩盖了数据有多旧。总览页在数据明显陈旧（超过三个采集周期）时额外提示「· 正在刷新…」，明确区分「这是落库快照」与「这是实时数据」。
- **Docs**: PRD §6.3 补充「快照落库（冷启动秒开）」与「速率类派生指标」两节，明确压缩、不可达落库、计数倒退、窗口标注等行为；§9 需求表新增「快照落库」「速率样本环」两行；**修订 §11 原则**——「轻量运维信号」的准入条件从「不引入任何历史存储」收紧为「只保留有界的、服务于就地差分的短期样本环（保留期即计算窗口），不提供任意时间范围的历史查询、不做趋势图与告警」。ARCHITECTURE 同步补充两张新表与数据流说明。
- **Backend**: 请求层补齐**多 host 故障转移**（`internal/es/client.go`）—— 新增 `Target.Do`，依次尝试集群的全部 host，只有传输层错误（连不上 / 超时）才切换，ES 返回的 4xx/5xx 原样透出、不重试；配置里非法或空白的 host 会被跳过；用一次请求记住「最近可用的 host」并在后续请求中优先命中，避免每次先撞一次失联节点，同时不主动切回抖动节点。全失败时报「集群的 N 个 host 均不可达」，不再把首个节点失联误判为整个集群不可达。`Probe` / `IndexCount` / 全部 `_cat` 拉取因共用 `getJSON` 而一并受益。
- **Backend**: 通用代理 `POST/GET /api/proxy/*path` 同样接入故障转移 —— 请求体在 1 MiB 以内时缓冲以便在多个 host 上重放，超过该上限（如 `_bulk` 大写入）则退化为「只打首个 host」，不把大请求整体吃进内存。
- **Backend**: 新增索引别名采集 kind `aliases`（`_cat/aliases`）与只读接口 `GET /api/clusters/:id/aliases`（返回 `{ data, updated_at }`，别名数量有上界故不分页）；前端复用同一套按需激活的共享采集，仅在该视图打开时才采集。`is_write_index` 等新列不被老版本 ES 识别时自动降级重试，守住 6.x / 7.x / 8.x 兼容。
- **Frontend**: 索引页 `ClusterIndices.vue` 增加「索引 / 别名」视图切换 —— 别名视图展示 别名 / 索引 / 写别名 / Filter / Routing，支持本地过滤与「别名数 · 覆盖索引数 · 映射条数」概览；点索引名即切回索引视图并预置过滤条件（同一页内跳转，不新增导航项）。
- **Frontend**: 索引页与分片页新增「隐藏 `.` 开头的索引」开关（默认开启，同 Cerebro）—— 系统索引（`.security-7` / `.kibana_1` 之类）默认不再占用视线。索引页把过滤下推到后端 `GET /api/clusters/:id/indices?hide_system=1`，`total` 与分页仍准确（`internal/handlers/catalog.go`）；分片页只过滤矩阵的索引轴，顶部汇总仍按整个集群统计并标注「已隐藏 N 个系统索引」；空列表会说明「. 开头的索引已隐藏」，从别名视图点进系统索引时自动关掉该开关，避免跳过去看到空表。
- **Docs**: 重写 PRD §2.4 为「单页看板：信息密集、动态刷新、不增栏目」—— 明确 ESPulse 是专业工具而非消费级产品，判断标准是「能否不切换页面就把状态看全」；指标要多且可横向对比，数据要像行情看板一样自己跳动；新能力一律并入既有页面，方向是继续缩减页面数量；点明反面教材是 Kibana、正面参照是 Cerebro。
- **Docs**: 据 §2.4 同步 PRD §3 / §6.1 / §6.4 —— **取消节点列表独立页**（节点信息并入总览节点表，成为节点信息的唯一去处），别名并入索引页视图；§9 补充多 host 故障转移行为，§10 移除已解决的「多 host 故障转移」待定项（保留多实例下的跨进程共享问题）。
- **Docs**: 同步 PRD（Phase 1 索引列表行、§6.2 过滤说明）与 TASKS 看板 —— 记录「默认隐藏 `.` 开头的系统索引」，并明确分片页该开关只作用于索引轴、顶部汇总仍是整个集群的数字。
- **Docs**: TASKS 看板同步 —— 多 host 故障转移与索引别名移入已完成，节点列表标注为「并入总览、取消单独设页」。

#### Dev Console 编辑保持
- **Frontend**: Dev Console 编辑器内容**按集群持久化**（`espulse:console-code:<clusterId>`，localStorage）—— 此前内容只存在组件内，`keep-alive` 复用同一实例导致 A 集群的草稿串到 B 集群，且刷新即丢。现在切集群先落盘旧草稿、再载入新草稿，刷新 / 重开自动恢复；写入做 300ms 防抖并在 `onDeactivated` / `onBeforeUnmount` 强制落盘（与 Kibana Console 历史存 localStorage 的思路一致）。`response` 不持久化，切集群时一并清空上一集群的执行结果，避免出现「B 集群的编辑 + A 集群的结果」这种不一致。

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
