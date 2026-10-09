# TASKS — ESPulse 任务看板

> 状态：进行中 | 当前阶段：**Phase 1 - Cerebro 核心功能替代**
> 原则：功能先行，UI 后置（见 [PRD.md](./PRD.md) §2.1）；单页看板、信息密集、动态刷新、不增栏目（见 §2.4）

---

## 🎯 当前目标

对齐 Cerebro 核心功能，做到可以日常替代使用。

---

## 🛠 正在进行 (In Progress)

### 1. 集群连接管理（含连通性探测）
- [x] 集群 CRUD 接口（List / Create / Update / Get / Delete）
- [x] 抽出可复用的 ES 请求层 `internal/es/client.go`
- [x] `Probe` 探测函数（`GET /` + `GET /_cluster/health`）
- [x] `POST /api/clusters/probe`：保存前探测，不落库
- [x] 强制「先探测、后保存」（新增、以及编辑时连接信息变更）
- [x] 前端：Cluster Hub 卡片墙（空状态 / 新增 / 编辑 / 删除二次确认 / 手动刷新）
- [x] 移除顶部集群选择器，活动集群改由 URL 决定
- [x] 多 host 故障转移（请求层依次尝试各 host；只有传输层错误才切换；记住最近可用节点并优先命中；全失败时报「N 个 host 均不可达」）
- [ ] 列表 / 表格视图切换（已归档到 PRD §8 功能池）

### 2. 集群健康采集
- [x] `internal/es/collector.go`：按需激活的共享采集（5s 间隔 + 失败指数退避）+ 内存快照
- [x] `GET /api/clusters` 返回时附带 status（reachable / health / version / node_count / …）
- [x] 添加 / 编辑集群后立即采集一次，避免等待下个周期
- [x] `POST /api/clusters/refresh`：手动触发重采
- [x] 采集器改造为「按需激活的共享采集」：按 `(集群, kind)` 共享缓存 + 单飞去重 + `lastAccess` 空闲淘汰（见 PRD §6.3）
- [x] 扩展 kind：`nodes`（`_cat/nodes`）、`shards`（`_cat/shards`）、`indices`（`_cat/indices`）、`aliases`（`_cat/aliases`）
- [x] 数据接口：`GET /api/clusters/:id/{nodes,indices,shards,aliases}`，统一返回 `{ data, updated_at }`
- [x] 快照落库（`snapshots` 表，gzip 压缩）：内存被空闲淘汰 / 进程重启后先渲染落库快照再后台刷新，只有「从未采集过」的集群才同步等一次采集
- [x] 速率样本环（`index_samples` 表）与索引写入 / 搜索速率（就地差分，窗口 60s，随快照返回 `rate_window_ms`）
- [ ] 多实例部署下的跨进程采集去重（见 PRD §10）

### 3. Cerebro 核心页面（数据优先，UI 后置）
- [x] 集群总览：状态仪表盘 + 问题清单（见 PRD §6.1）
  - [x] 健康条：status / active_shards_percent / 未分配 / relocating / initializing
  - [x] 关键计数：节点数 / 索引数 / 主副分片数 / 文档总数 / 存储总量
  - [x] 节点简表：name / role / master / heap% / cpu% / disk% / 承载分片数
  - [x] 问题清单：未分配分片 Top 20（含原因）/ relocating 分片 / 非 green 索引（失衡改由节点表的「分片」列呈现，不做模糊阈值判定）
  - [x] 数据接口 `GET /api/clusters/:id/overview`（聚合 status / nodes / indices / shards，支持 `?refresh=1` 强制重采）
- [x] 节点信息并入总览的节点表（**取消单独设页**，见 PRD §2.4 / §6.4）
- [x] 索引列表：`_cat/indices`，后端过滤（索引名 / health / status / 隐藏 `.` 开头索引）+ 排序（索引名 / 存储 / 文档 / health / 写入速率 / 查询速率）+ 分页，前端表格与分页器；侧边栏新增 Indices 入口（`/cluster/:id/indices`）
- [x] 索引列表展示写入 / 搜索速率（样本不足显示「—」，列头提示差分窗口；见 PRD §11）
- [x] 分片分布：`_cat/shards` 矩阵，支持「按节点 / 按索引」视角切换、索引名搜索、「隐藏 `.` 开头的索引」（默认开启）与「只看有问题的索引」过滤、索引轴分页（见 PRD §6.2）；侧边栏新增 Shards 入口（`/cluster/:id/shards`）
- [x] 索引别名：`_cat/aliases`，后端 kind 采集 + `GET /api/clusters/:id/aliases`，前端并入索引页的「别名」视图（**不单独设页**，见 PRD §2.4）

---

## ⏭ 后续阶段

### Phase 2 — Dev Console
- [x] Monaco Editor 集成与真实请求发送
- [ ] 命令目录侧边栏：解析当前编辑器命令行生成大纲，点击定位到对应行
- [ ] 多 Tab 并行编辑
- [ ] ~~请求历史（最近 200 条）~~ 暂缓，后续再评估
- [ ] ~~命令收藏夹与变量占位符~~ 暂缓，后续再评估

### Phase 3 — 差异化探索（待定）
- [ ] 差异化押注确定（见 PRD §10）
- [ ] 多集群统一视角
- [ ] 跨集群执行对比

### 平台能力
- [ ] `embed.FS` 内嵌前端 + 单二进制构建脚本
- [ ] Electron 打包（Windows / macOS / Linux）
- [ ] 配置文件预置集群
- [ ] 集群凭据 AES-GCM 加密存储

---

## ✅ 已完成 (Done)

- [x] 前端：Dev Console 基础版（Monaco 集成、ES 语法高亮与补全、真实请求发送）
- [x] 前端：集群选择与全局状态管理（Pinia + LocalStorage）
- [x] 前端：基础布局（侧边栏、顶部集群选择器、UnoCSS 主题）
- [x] 后端：ES 通用代理 `/api/proxy/*path`（含 Basic / ApiKey 认证）
- [x] 后端：集群 CRUD（List / Create / Get / Delete）
- [x] 后端：SQLite 初始化与建表迁移
- [x] 文档：PRD / ARCHITECTURE / TASKS / CHANGELOG 体系
- [x] 工程：Air 热重载、.gitignore、.gitkeep、pnpm + Vite 代理
