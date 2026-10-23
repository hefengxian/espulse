# CHANGELOG — ESPulse 变更日志

> 记录项目的每一个足迹。

---

## [Unreleased]

### 2026-10-23

#### 索引网格：主/副、文档、存储改为独立列
- **Frontend**: 原本挤在索引名右侧的元信息微列拆成三个表格列（索引列由 326px 收窄到 300px），列头「文档」「存储」可直接点击排序；组头行在同一列展示组内汇总值，行宽标记 `N×` 留在组名前缀旁。

#### 统一 Hub 与工作台的色板与顶栏
- **Frontend**: 色板收敛为单一真相源 —— 工作台不再自带一套 hex，`.esp-wb` 的 `--bg/--text/--green/...` 全部指向 `:root` 的 `--esp-*`；补齐 `--esp-hover` / `--esp-text-4` / `--esp-blue`，删除未使用的 `--accent-2` 与 `--violet-*`。工作台原先的亮绿 `#3ddc97` 一并收敛为 `#22c55e`：浅色主题下 6px 健康点更可见，且与 initializing 的蓝色色差更大。工作台原先只覆盖一半令牌的浅色块随之删除，明暗完全交给 `:root.light`；Monaco 主题的字面量同步到新色板。
- **Frontend**: Hub 顶栏由 52px 收到 44px（内边距 16→12px、logo 28→23px），与工作台顶栏对齐。
- **Frontend**: ClusterHub 的 Naive UI 控件统一取 `small` 档（按钮 / 输入 / 下拉 / 表单 / 卡片 / Popconfirm 一致），表单尺寸由 `n-form` 统一下发。`tiny` 的 12px 字号低于卡片正文，放在 16px 内边距的卡片里会被读成标签而非按钮，因此不作为 Hub 的下限。

### 2026-10-22

#### 修复：无索引时节点视角不显示节点
- **Frontend**: 网格区空态原来只看过滤后的索引数，集群没有索引时会把节点视角一并判成空态；改为按当前视角取行数（节点视角看 `overview.nodes`），节点视角的空态文案单独走 `grid.emptyNoNode`。

#### 健康环形图跟随空集群状态
- **Frontend**: 环形图原先只按索引健康计数画占比，新建的空集群三项计数都是 0，会落成满圈红；索引数为 0 时改用集群自身的 `health.status`，绿色空集群显示绿圈。

#### 节点视角补充地址与堆上限
- **Backend**: `_cat/nodes` 增加 `ip` / `port` / `heap.max` 列，`overview.nodes` 相应带上 `ip`、`port`、`heap_max`，不新增请求。
- **Frontend**: 节点名下方第二行由角色改为 `ip:port`（角色从各列已能判断）；Heap 列在百分比左侧显示堆上限，列宽由 52px 增至 96px。

### 2026-10-21

#### 品牌 Logo
- **Frontend**: Hub 顶栏与工作台顶栏的占位圆心替换为「ES 三段标记 + 心跳线」的 Logo，图形走透明底，尺寸与底色交给容器；工作台 `.mark` 相应去掉渐变底与光晕。
- **Assets**: 设计过程中的 v1–v8 方案保留在 `frontend/public/`，供后续按集群状态换心跳色时取用。

#### 节点视角资源用量可视化
- **Frontend**: 节点视角的 Heap / CPU / 磁盘由纯百分比改为「数字 + 3px 用量轨」：轨道长度即使用率，颜色按原有阈值分级（绿 = 正常，黄 = 高负载，红 = 危险）；轨道压在数字下方，行高不变，列宽不变。常态绿轨压到 55% 不透明度，只有黄 / 红才同时染数字，避免三列彩色数字抢读。
- **Frontend**: 缺值显示 `-` 并留空轨，不再出现 `-%`；轨道宽度做 0–100 夹取，脏数据不会撑破单元格。

#### 索引网格标记「副本 0」
- **Frontend**: 索引行的 `主/副` 微列在副本为 0 时转黄、副本数字加虚线底线，`title` 补充「无冗余，节点故障即可能丢数据」。用黄色而非红色区分「冗余缺口」与「已经不可用」；只染这一格、不加行底纹 —— 日志类索引常有意不建副本，行级高亮会让整张表变成噪声。
- **Frontend**: 前缀归并的组头单独统计组内无副本索引数，任一子索引缺副本即标记并在 `title` 给出数量 —— 组头的 `rep` 是求和，个别索引 0 副本会被掩盖。close / frozen 索引不参与告警。

### 2026-10-19

#### 指标条补充 Pending Tasks
- **Backend**: `status` / `overview.health` 带上 `_cluster/health` 的 `number_of_pending_tasks`（`Status.PendingTasks` → `overview.health.pending_tasks`），沿用同一份 5s 状态快照，不新增 ES 请求。
- **Frontend**: 指标条「恢复」组增加「待处理任务」；数据未到时与其他指标一样显示 `—`，不摆 0。
- **Docs**: PRD §6.1 关键计数补充待处理任务数。

#### 节点视角：Master 列并入节点名，加宽负载列
- **Frontend**: 去掉独立的 Master 列（含表头与单元格），主节点改为在节点名后紧跟 15px 黄色星标；表头 colspan 由 8 调整为 7。
- **Frontend**: 负载列由 120px 加宽到 156px —— 11px 等宽字宽约 6.6px，`99.99 / 99.99 / 99.99`（21 字）约 138.6px，加左右各 8px 内边距约 154.6px，原宽度会把第三段裁掉。

#### 索引视角只展示 data 节点
- **Frontend**: 索引视角的节点列按 `node.role` 过滤，只保留含 `d/h/w/c/f/s` 任一角色码的 data 节点 —— master-only / coordinating 节点没有分片，占一整列只会挤压分片矩阵。不能只看 `d`：分层集群的数据节点角色码是 `h` / `w` / `c` / `f` / `s`。没有分片的 data 节点仍然保留，不做分片计数。
- **Frontend**: 节点视角不变，继续展示全部节点，指标条的节点计数也不受影响。
- **Docs**: PRD §6.2 / §6.5 记录两视角的节点口径；TASKS 勾掉该项。

### 2026-10-16

#### 分片恢复进度（`_recovery`）
- **Backend**: 新增采集 kind `recovery`（`GET /_recovery?active_only=true`，5s 间隔，共享缓存 / 单飞去重 / 指数退避 / 快照落库与其余 kind 一致），落接口 `GET /api/clusters/:id/recovery`。响应把 ES 按索引分组的嵌套结构拍平成「一行一个分片副本」（`primary` / `stage` / `source` / `target` / 类型 / 文件 / 字节 / translog / 耗时），百分比一律不取 ES 的字段（6.x 给的是 `"87.1%"` 字符串），改由前端按字节自算。
- **Backend**: 采集选 `_recovery` 而不是 `_cat/recovery`：只有前者带 `primary`，且一个分片副本恰好一条记录；`_cat/recovery` 没有主副标识，一个分片还可能因为参与多种恢复类型而出现多行。`active_only=true` 不能省 —— 默认会把集群里**每一个分片副本**的最后一次恢复都报出来（万级索引下是 MB 级响应）。PRD §9 的「只用 `_cat/*`」据此补上 `_recovery`。
- **Backend**: `GET /api/clusters/:id/overview` 增加 `version`（取自同一份 status 快照）。集群版本是静态身份，此前只在 Hub 卡片上出现，工作台看不到。
- **Frontend**: 指标条补上集群版本，叠在健康胶囊正上方并居中（借那一格的竖向空间，不新占横向位置）；「恢复吞吐 / 剩余」由留白改为真实值 —— 吞吐 = Σ(单个分片的已恢复字节 / 它自己的耗时)，并行恢复时这才是真实总吞吐（用 Σ字节 / Σ耗时 会随并行度被稀释），剩余 = 按该吞吐跑完剩余字节的 ETA。只在有恢复时出数，空闲时留空而不摆 0（0 会被读成「恢复停滞」）。口径是「平均速率」而非窗口差分，因此不需要样本环；代价是它对「先快后停」的恢复不敏感，判断某个恢复是否停住交给按耗时降序的表与它自己的进度。
- **Frontend**: 活动区「分片恢复 / 搬迁」由 5 列扩到 8 列 —— 分片改为复用网格那套方块（实心 = 主 / 描边 = 副，编号写在方块里；不上状态色，`_recovery` 并不知道分片的 routing 状态，状态语义归「阶段」列），另加阶段 / 源 → 目标 / 文件 / 字节 / translog 进度 / 耗时。**恢复类型不单独占列**（它多数时候就是 PEER，等于没信息；只有节点重启、快照恢复时才解释得通整屏），移入方块 `title`。表按**耗时降序**、不截断行数（恢复是瞬时状态，平时表是空的；行多时活动区自己 40vh 内滚动），进度条只画快慢、绝对值（`12 / 900`、`1.2 GB / 3.5 GB`）放 `title`，不把每行撑成两行高。计数与列表改为同源（都用 `_recovery`），不再用 `_cat/shards` 的 relocating / initializing 另拼一份，避免出现「计数 3、表格空」的矛盾。`_recovery` 只回答「此刻」，恢复完成的瞬间那一行就消失，因此这里不保留完成留痕。
- **Frontend**: 活动区展开态去掉了「活动中」标签与闪动黄点 —— 它和状态栏的 `live` 点是同一个信号的两次表达，而活动区展开本身已经说明「有活动」；左侧只留计数，与收起态同构。
- **Docs**: PRD §6.1（活动区职责与恢复表、指标条补版本）、§6.3（kind 表）、§6.4（数据源）、§9（版本兼容口径），ARCHITECTURE §2 / §3 / §4 同步；TASKS 勾掉该项。

#### 修复：跨集群的迟到响应污染工作台
- **Frontend**: 加载引入「代际」判定（`loadSeq`）—— 切集群、手动刷新、轮询都会开启新的一代，只有最新一代的结果允许落盘，迟到的响应连同 `error` 一起丢弃。此前谁后返回谁覆盖：浏览大集群 A 时切到轻量的 B，A 的慢响应会在 B 渲染完之后回来，把节点 / 分片 / 索引换回 A 的，直到下一轮轮询才自愈。
- **Frontend**: 索引列表由「自己读 `clusterId` 再直接写状态」改为纯取数（`fetchIndices`），由 `loadAll` 校验代际后统一落盘 —— 切换发生在 `await` 期间时，原先会出现 overview 取自 A、indices 取自 B 的混合状态。分片分配开关的读写同样纳入代际判定，乐观更新的失败回滚也只在集群未变时才执行。
- **Frontend**: 切集群时立即清空上一个集群的数据并作废在途请求（离开页面时同样作废）。PRD §2.5 增加「数据永远属于当前集群」规则。

#### 工作台的等待态：骨架屏与空态
- **Frontend**: 网格区区分「等待数据 / 确实没有数据 / 加载失败」三态：等待期显示骨架屏（沿用网格的列宽与行距节奏，数据到位后不跳版），空态区分「该集群暂无索引」与「没有匹配的索引（. 开头的索引已隐藏）」。此前三种情况都只是一张空表，仅有一行「加载中…」灰字，且因旧数据从不被清空，切集群时连那行灰字都不会出现。
- **Frontend**: 数据未到时指标条与活动区降级为占位（数字显示 `—`、健康 chip 与环形置灰、活动区显示「加载中」），不再摆 0 与默认 GREEN —— 那会被读成「集群很健康」。PRD §2.5 增加「等待期不留白」规则。

#### 修复：索引元信息列定宽过窄导致截断
- **Frontend**: `.ixm.pr / .docs / .store` 的定宽由 48 / 42 / 38px 调整为 40 / 46 / 45px —— 原值偏小，`docs` 只够 4 字、`store` 只够 4 字（10px 等宽下内容区仅 31px），`45.6k` / `1.2gb` 这类常见短值也被 `text-overflow: ellipsis` 吞掉。新值按 90% 场景反推：字宽 6px/字，中列 = 1px 描边 + 双侧 7px 内边距 = 15px，末列无右内边距 = 8px，取值上限 `pr` 4 字、`docs` 5 字、`store` 6 字，各留 1px 防亚像素溢出；组头行与子索引行仍成列对齐。

### 2026-10-15

#### 活动区头改为常驻
- **Frontend**: 活动区原先收起态与展开态各有一行头，字号还不一致（`.idle` 用 `--fs-ui`，`.eb-head` 的计数与时间用 `--fs-data`）。现合为一行的常驻头：左侧摘要随状态切换，右侧「健康 / 恢复」开关位置固定不动，两侧字号统一为 `--fs-ui`；摘要限宽裁剪，长文案不会把开关挤出视野。
- **Frontend**: 顶栏右簇因此收敛为「刷新 / 语言」——「健康 / 恢复」开关原在顶栏，但它只控制活动区收展，在控制台模式里是个看得见却点不动的控件。活动区两处的「更新于」也随之下线（右侧固定给开关）。PRD §6.1 同步。

#### 工作台数据新鲜度：移入状态栏 + 防抖
- **Frontend**: 「更新于 N 秒前」从顶栏移到网格状态栏右端（`live` 点与「恢复中 · 」一并跟随）。顶栏是工作台与开发控制台共享的，放在那里会让控制台也挂着一份用不到的时间；状态栏在控制台模式下整体 `visibility: hidden`，挪过去就只在工作台出现，顶栏右簇相应收敛为「健康 / 恢复 | 刷新 | 语言」。PRD §2.5 同步从「刷新按钮旁边常驻」改为「新鲜度是被动提示，不占顶栏」。
- **Frontend**: 原先它是一整串文本，`N` 从 1 位变 2 位（9→10）时整段宽一个字符，把相邻元素顶走。现由 `utils/freshness.ts` 的 `ageParts` 拆成「前缀 / 数字 / 单位」，数字用定宽槽（`.numslot`，`min-width: 2ch`）右对齐渲染；状态栏、活动区收起态与展开态三处一起生效。

### 2026-10-12

#### 工作台转正（v5 → 唯一落地页）
- **Frontend**: **统一工作台（`/workbench/:id`）正式转正为集群的唯一落地页**。此前它只是与旧版并存的独立页面（顶栏留了个入口），同一个集群有两套 UI、两处状态，任何改动都要维护两份。现在 Hub 的集群卡片、集群切换器、历史书签全部指向工作台；旧模块路径 `/cluster/:id/{overview,indices,shards,console}` 一律重定向到 `/workbench/:id`（书签 / 历史不做兼容页，避免出现「点进去看的是旧世界」的错觉）。
- **Frontend**: 删除旧版模块页 —— `views/ClusterOverview.vue` / `ClusterIndices.vue` / `ClusterShards.vue` / `DevConsole.vue`，以及只服务它们的 `composables/useViewFilters.ts`（「过滤条件按集群记忆」的载体）。旧三页的能力已在工作台里重建：分诊 = 指标条 + 活动区，索引清单 = 网格索引视角，分片分布 = 网格两视角，Dev Console = 顶栏模式切换。
- **Frontend**: 顶栏去掉模块 Tab —— 模块之间不再有跳转，只有工作台内的视角（索引 / 节点）与模式（工作台 / 控制台）切换，横向导航因此失去承载对象；`MainLayout` 收敛为 Hub 的管理页外壳。
- **Frontend**: Cluster Hub 整合 —— 集群卡片点击直接进工作台，不再跳旧版 Overview。
- **Frontend**: 工作台接管「无效集群 id 收敛回 Hub」的守卫 —— 这段逻辑原先在 `MainLayout` 里（它包裹旧模块页），而工作台是独立路由、不在其下；旧页删掉后，删集群留下的历史 / 书签会停在一个永远加载不出来的工作台上，因此把守卫移进工作台（集群列表加载完成后仍找不到该 id → 回 Hub）。
- **Docs**: PRD 转正 —— §2.5 重写为「Hub / 集群切换器 / 统一工作台」三层骨架（并说明为什么把 Overview + Indices + Shards 合成一页），§2.4 的「不新增栏目」改以「视角 / 模式切换不是栏目」表述，§3 典型一天、§6（Phase 1 表、6.1 指标条与活动区、6.2 统一网格、6.3 kind 归属、6.4 各区域职责边界）、§7 当前实施范围同步；TASKS 重排为「工作台转正 / 已知缺口 / 技术债 / 后续」；ARCHITECTURE 目录结构与路由同步。
- **Docs**: 显式记录两处**已知缺口**（属「还没做」，不是「不做」）：旧版索引页的**别名视图**尚未迁入工作台（`_cat/aliases` 采集与接口都还在），以及视角 / 过滤条件仍未 URL 化。
- **Frontend**: 去掉「并存时期」留下的 `v5` 后缀 —— `views/WorkbenchV5.vue` → `views/Workbench.vue`、`styles/workbench-v5.less` → `styles/workbench.less`、路由名 `WorkbenchV5` → `Workbench`。原型稿 `ui_prototype/espulse-workbench-v5.html` 是一组设计参考中的一版，名字保留。

### 2026-10-10

#### v5 工作台与地基
- **Frontend**: 新增 **v5 统一工作台**（`WorkbenchV5.vue` + `components/workbench/*` + `styles/workbench-v5.less`，约 3.5k 行）—— 把总览 / 索引 / 分片三页的信息压进一屏：**指标条**（health + 索引健康环 + active 占比 + 规模计数 + 分片构成堆叠条）、**活动区**（relocating / initializing / 未分配（含原因）/ 非 green 索引，带「健康 / 恢复」折叠开关，点索引名就地过滤）、**统一网格**（索引视角与节点视角一键切换，单元格 = 分片方块，未分配分片单列）、**Dev Console 模式**（顶栏切换，首次切入才挂载 Monaco）。样式作用域限定在 `.esp-wb`，与既有 UnoCSS 页面互不影响，因此当时可与旧页并存。
- **Frontend**: 顶栏加入工作台入口与集群内切换联想（`ClusterSwitcher` 支持 `/workbench/*` 路径原地换集群）。
- **Frontend**: 新增 `utils/format.ts`（字节 / 整数 / 速率 / 用量配色）作为展示层格式化工具。
- **Frontend**: **明暗主题抽取为 `useTheme` 组合式地基** —— 读取 localStorage 偏好、缺省跟随系统、应用到 `html.dark` / `.light`；`App.vue` 的主题色覆盖与背景改为按主题切换，`MainLayout` 不再内置主题逻辑。
- **Frontend**: **接入 vue-i18n 并新增语言切换地基** —— 新增 i18n 实例与 `zh-CN` / `en-US` 语言包（未声明的 key 回退中文），`useLocale` 封装切换并持久化到 localStorage。
- **Backend**: **放宽探测与目录客户端超时** —— `ProbeClient` 5s→10s（它同时承载交互式「测试连接」，超时过大时连不上的集群要等很久），`CatalogClient` 30s→90s（万级索引下 `_cat/indices` 耗时可能远超探测超时）。
- **Docs**: 补充 v5 工作台 UI 原型稿（`frontend/ui_prototype/espulse-workbench{,-v2,-v3,-v4,-v5}.html` 与参考图）。原型页是独立 HTML（Google Fonts 引入、不被构建引用），仅作设计参考保留。

#### Dev Console 与规划
- **Frontend**: 移除 Dev Console 命令目录（Navigator）里 `Saved` / `History` 两个**占位 Tab** —— 此前点击只切换高亮、内容恒为「This session」的命令大纲（无对应实现），属误导性 UI；连带删除无用的 `activeNavTab`。
- **Docs**: 同步项目状态 —— TASKS 将「命令目录侧边栏」标记为已完成、删除「多 Tab 并行编辑」，并记录占位 Tab 的移除；PRD Phase 2 移除「多 Tab」。
- **Docs**: 明确优先级 —— Dev Console「**智能提示**」写入规划，定为后续**重点增强**方向（PRD §6 Phase 2 / TASKS Phase 2）；打包与多平台发布独立为**最后环节（Phase 4 / 「最后环节 — 打包与多平台发布」）**，并声明「所有设计功能完成前不考虑实现」，PRD §8 平台能力同步标注。

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
- **Frontend**: Monaco 编辑器**字号 / 行高由 13px / 22px 收敛为 12px / 18px**，与命令目录（Navigator）的 `text-12px`（根 `line-height: 1.5` → 18px）对齐 —— 字族本就同为 JetBrains Mono，此前只是编辑器大一号、行距更松，两者观感割裂。结果面板复用 `editorOptions`，一并生效。
- **Frontend**: 结果面板（Response）**初始为空** —— 去掉此前硬编码的示例 JSON，进入页面不再展示假数据（`resultLanguage` 对空串回落到纯文本，不影响高亮逻辑）。
- **Frontend**: 结果面板顶部新增**复制按钮** —— 一键把返回结果写入剪贴板，成功后图标短暂变为对勾作反馈；结果为空时按钮置灰禁用。
- **Frontend**: 移除编辑器头部的 `Format` 按钮，改为 **Monaco 原生 action** —— 右键菜单「Format Document」+ 快捷键 `⇧⌥F`（Mac）/ `Shift+Alt+F`。文档级操作不再独占头部栏，头部回归「文件名 + 命令数」。
- **Frontend**: 编辑区与结果面板的**头部栏统一为固定 32px（`h-8`）并垂直居中** —— 此前两端靠各自 `p-2.5` 撑高，且撑高来源不同（编辑器是文件名文字、结果面板是复制按钮），高度差约 5px，横向 padding 也不一致（`px-4` vs `px-3.5`）。现抽成共享 shortcut `panel-header`（`h-8 flex items-center px-3 border-b border-border bg-bg-2`），两端只写差异部分，避免再次漂移。

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
