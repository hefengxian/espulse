# ESPulse — 架构设计文档

> 版本：v0.2 | 对应 PRD：v0.2

---

## 1. 整体架构

```
┌─────────────────────────────────────────────────────┐
│                    前端 (Vue 3)                       │
│  ┌─────────────┐   ┌────────────────────────────┐   │
│  │ Cluster Hub │   │ 工作台 /workbench/:id        │   │
│  │  管理与浏览  │   │ 指标条·活动区·统一网格·控制台 │   │
│  └─────────────┘   └────────────────────────────┘   │
│              Naive UI + Pinia + Vue Router            │
└────────────────────┬────────────────────────────────┘
                     │ HTTP (REST API)
┌────────────────────▼────────────────────────────────┐
│                后端 (Go + Gin)                        │
│                                                       │
│  /api/clusters/*      集群 CRUD + 探测 + 健康采集      │
│  /api/proxy           ES 请求转发（Dev Console 核心）  │
│  /api/clusters/:id/*  聚合数据（overview / nodes /    │
│                       indices / shards / aliases）    │
│  /*                   静态资源（embed.FS 内嵌前端）    │
│                                                       │
│  ┌─────────────────────────────────────────────┐     │
│  │           SQLite (modernc.org/sqlite)        │     │
│  │      clusters / snapshots / index_samples    │     │
│  └─────────────────────────────────────────────┘     │
└────────────────────┬────────────────────────────────┘
                     │ HTTP（直接调用 ES REST API）
         ┌───────────┼───────────┐
         ▼           ▼           ▼
      ES 6.x      ES 7.x      ES 8.x
     Cluster A   Cluster B   Cluster C
```

**关键设计：前端静态资源通过 `embed.FS` 内嵌进 Go 二进制**，服务器模式下只需分发一个文件，无需 Nginx 或独立的静态文件服务。

---

## 2. 目录结构

```
espulse/
├── main.go                      # 入口：加载配置、初始化 DB、启动 Gin
├── go.mod
├── go.sum
│
├── internal/
│   ├── config/                  # 预留：端口 / 数据目录等配置（当前为空，实现未排期）
│   │
│   ├── database/
│   │   ├── db.go                # SQLite 连接初始化（modernc.org/sqlite + database/sql）
│   │   └── migrate.go           # 建表 / 版本迁移（clusters / snapshots / index_samples）
│   │
│   ├── models/
│   │   └── cluster.go           # Cluster 结构体 + CRUD
│   │
│   ├── handlers/
│   │   ├── clusters.go          # 集群 CRUD + probe / refresh
│   │   ├── overview.go          # GET /api/clusters/:id/overview（聚合 status+nodes+indices+shards）
│   │   ├── catalog.go           # nodes / indices / shards / aliases（后端过滤 + 排序 + 分页）
│   │   ├── catalog_test.go
│   │   └── proxy.go             # POST /api/proxy  固定信封的 ES 请求转发
│   │
│   ├── es/
│   │   ├── client.go            # ES 客户端（SharedClient / ProbeClient / CatalogClient）+ 多 host 故障转移
│   │   ├── cat.go               # _cat/*?format=json 解析与索引过滤 / 排序
│   │   ├── collector.go         # 按需激活的共享采集（缓存 + 单飞去重 + lastAccess 淘汰）
│   │   ├── rate.go              # 索引写入 / 搜索速率的就地差分（样本环，窗口 60s）
│   │   ├── snapshot_store.go    # 快照落库（gzip）与样本读写
│   │   └── collector_test.go
│   │
│   └── router/
│       └── router.go            # Gin 路由注册 + 静态资源 embed 挂载
│
├── frontend/                    # Vue 3 前端（独立开发，构建产物内嵌进 Go）
│   ├── src/
│   │   ├── main.ts
│   │   ├── App.vue
│   │   ├── router/index.ts      # `/`（Hub）与 `/workbench/:id`（工作台）；旧 `/cluster/:id/*` 重定向到工作台
│   │   ├── layouts/
│   │   │   └── MainLayout.vue   # Hub 的外壳（品牌 + 集群切换器 + 刷新 / 主题），工作台自带顶栏不用它
│   │   ├── views/
│   │   │   ├── ClusterHub.vue   # 集群管理 + 浏览（增删改、0 集群引导、卡片墙）
│   │   │   └── Workbench.vue    # 统一工作台：指标条 + 活动区 + 统一网格 + Dev Console 模式
│   │   ├── components/
│   │   │   ├── ClusterSwitcher.vue
│   │   │   └── workbench/
│   │   │       ├── WorkbenchGrid.vue     # 索引视角 / 节点视角的统一网格
│   │   │       ├── WorkbenchConsole.vue  # Monaco + 命令目录 + 请求执行
│   │   │       └── types.ts
│   │   ├── composables/         # useTheme / useLocale / useGlobalRefresh
│   │   ├── i18n/                # index.ts + locales/{zh-CN,en-US}.ts
│   │   ├── stores/              # cluster.ts（集群列表与状态）、metadata.ts（控制台补全用的索引 / 字段元数据）
│   │   ├── styles/              # workbench.less（作用域限定 .esp-wb）
│   │   ├── utils/               # format.ts（字节 / 整数 / 速率）、freshness.ts（更新于 N 秒前）
│   │   └── api/                 # clusters.ts / catalog.ts / overview.ts
│   ├── ui_prototype/            # 设计参考用的独立 HTML 原型（不被构建引用）
│   ├── package.json
│   └── vite.config.ts
│
├── scripts/
│   └── docker-es-test.yml       # 本地 ES 测试集群
│
└── api/
    └── clusters.http            # 手工验证用的 HTTP 请求集
```

> 前端页面骨架（层级、Hub 与工作台的职责、切集群与刷新规则）见 [PRD.md](./PRD.md) §2.5；工作台各区域的职责边界见 §6.4。

---

## 3. 数据库设计（SQLite）

使用标准 `database/sql` 接口 + `modernc.org/sqlite` 驱动（纯 Go，无 CGO）。

```sql
CREATE TABLE IF NOT EXISTS clusters (
    id          TEXT PRIMARY KEY,       -- UUID v4
    name        TEXT NOT NULL,          -- 显示名称，如 "生产集群 A"
    hosts       TEXT NOT NULL,          -- JSON array: ["http://es1:9200","http://es2:9200"]
    auth_type   TEXT DEFAULT 'none',    -- none | basic | api_key
    username    TEXT DEFAULT '',
    password    TEXT DEFAULT '',        -- AES-GCM 加密存储
    api_key     TEXT DEFAULT '',        -- AES-GCM 加密存储
    color       TEXT DEFAULT '#18a058', -- UI 标识色
    notes       TEXT DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS snippets (
    id          TEXT PRIMARY KEY,
    cluster_id  TEXT,                   -- NULL 表示通用片段
    title       TEXT NOT NULL,
    method      TEXT NOT NULL,          -- GET | POST | PUT | DELETE | HEAD
    path        TEXT NOT NULL,          -- 如 /_cat/indices
    body        TEXT DEFAULT '',
    category    TEXT DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS console_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    cluster_id  TEXT NOT NULL,
    method      TEXT NOT NULL,
    path        TEXT NOT NULL,
    body        TEXT DEFAULT '',
    status_code INTEGER,
    duration_ms INTEGER,                -- 请求耗时
    executed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 每个 (集群, 数据类型) 的最新一份采集快照。data 是 gzip 后的 JSON。
-- 它让「空闲被淘汰 / 进程重启」的集群也能秒开：先渲染旧快照，再后台刷新（见 PRD §6.3）。
CREATE TABLE IF NOT EXISTS snapshots (
    cluster_id TEXT NOT NULL,
    kind       TEXT NOT NULL,           -- status | nodes | indices | shards | aliases
    data       TEXT NOT NULL,           -- gzip(JSON)
    updated_at INTEGER NOT NULL         -- Unix 毫秒，即该快照的采集时间
    PRIMARY KEY (cluster_id, kind)
);

-- 索引累计计数的短期样本环，只服务于写入 / 搜索速率的就地差分（见 PRD §11）。
-- at 排在 index_name 之前，「按时间裁剪」可直接命中主键前缀。
CREATE TABLE IF NOT EXISTS index_samples (
    cluster_id   TEXT    NOT NULL,
    at           INTEGER NOT NULL,      -- Unix 毫秒
    index_name   TEXT    NOT NULL,
    idx_total    INTEGER NOT NULL,      -- _cat/indices 的 indexing.index_total
    search_total INTEGER NOT NULL,      -- _cat/indices 的 search.query_total
    docs_count   INTEGER NOT NULL,
    PRIMARY KEY (cluster_id, at, index_name)
) WITHOUT ROWID;
```

---

## 4. API 设计

### 集群管理
```
GET    /api/clusters                    列出所有集群（含实时健康状态）
POST   /api/clusters                    添加集群（保存前做连通性测试）
POST   /api/clusters/probe              单独探测（不落库，保存前强制走这一步）
POST   /api/clusters/refresh            手动触发当前集群重采（绕过刷新间隔，仍走单飞去重）
GET    /api/clusters/:id                获取单个集群详情
PUT    /api/clusters/:id                更新集群配置
DELETE /api/clusters/:id                删除集群（连带清掉快照与速率样本）
```

### ES 请求代理（Dev Console 核心）
```
POST   /api/proxy
Header: X-Cluster-ID: <cluster_id>

Request body:
{
  "method": "GET",
  "path":   "_cat/indices?v&format=json",
  "body":   ""
}

Response:
{
  "status":      200,
  "duration_ms": 42,
  "body":        { ...ES 原始响应... }
}
```
请求采用**固定 POST + JSON 信封**（而不是把方法 / 路径编码进 URL）：集群 id 走 `X-Cluster-ID`，`method` 只允许 ES 语义下的安全集合，`body` 是原样透传的字符串。后端不做任何解析或封装，保持与 ES 的完全兼容。

### 集群数据接口
```
GET    /api/clusters/:id/overview                    工作台聚合数据（status+nodes+indices+shards），?refresh=1 强制重采
GET    /api/clusters/:id/nodes
GET    /api/clusters/:id/indices?search=&health=&status=&hide_system=1&sort=&order=&page=&page_size=
                                                     后端过滤 + 排序 + 分页，返回 { data, total, page, page_size, updated_at }
GET    /api/clusters/:id/shards
GET    /api/clusters/:id/aliases
```

**采集模型**：采集器按 `(集群, 数据类型)` 维护共享缓存，采用「需求驱动 + 单飞去重」——只有被页面订阅的数据类型才会被定时刷新，同一缓存项的并发请求只触发一次 ES 调用，`lastAccess` 超过空闲阈值的缓存项停止刷新并被淘汰。接口语义是「读缓存（必要时触发刷新）」，而非「每次请求都采集」，因此服务器模式下 N 个并发用户只对应一份采集。数据类型、刷新间隔与生命周期详见 PRD §6.3。

每轮采集成功后快照立即落库（`snapshots`，gzip 压缩），因此内存缓存被淘汰、或进程重启后，读取会先返回落库的历史快照再后台刷新——只有「从未采集过」的集群才需要同步等一次采集。索引的写入 / 搜索速率是就地差分出的派生值（`index_samples` 样本环，窗口 60s），随快照一并返回并标注实际窗口。

### 静态资源（服务器模式）
```
GET    /*    由 embed.FS 提供前端静态文件，所有非 /api 路由返回 index.html（SPA 模式）
```

---

## 5. ES 多版本兼容策略

**核心原则：不使用 ES 官方 Go 客户端（`elastic/go-elasticsearch`），直接用标准 `net/http` 发送请求。**

原因：ES 官方 Go 客户端与 ES 版本绑定，多版本场景需要维护多个客户端实例。使用原生 HTTP 直接调用 REST API 天然兼容所有版本。

兼容处理集中在 `internal/es/client.go`：
- ES 8.x 默认启用 HTTPS，连接时根据 hosts scheme 自动判断是否跳过 TLS 验证（可配置）
- ES 7.x 起废弃 `_type`，`_cat` API 跨版本 URL 一致，无需额外处理
- 版本探测：连接时调用 `GET /` 获取 ES 版本号，存入连接池供后续逻辑参考

---

## 6. 前端静态资源内嵌

```go
// main.go 或 router/router.go

//go:embed static/dist
var staticFiles embed.FS

func SetupRouter() *gin.Engine {
    r := gin.Default()

    // API 路由
    api := r.Group("/api")
    // ... 注册 handlers

    // 前端静态资源（SPA fallback）
    r.NoRoute(func(c *gin.Context) {
        // /assets/* 直接返回文件
        // 其他路径返回 index.html
        c.FileFromFS("static/dist/index.html", http.FS(staticFiles))
    })

    return r
}
```

---

## 7. 构建流程

```bash
# scripts/build.sh

# 1. 构建前端
cd frontend && npm run build    # 输出到 ../static/dist/

# 2. 编译 Go（关闭 CGO，确保跨平台）
cd ..
CGO_ENABLED=0 go build -ldflags="-s -w" -o espulse ./main.go
```

```bash
# scripts/build-all.sh（交叉编译三平台）

CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o dist/espulse-linux-amd64   .
CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -o dist/espulse-darwin-amd64  .
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o dist/espulse-darwin-arm64  .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/espulse-windows-amd64.exe .
```

---

## 8. Electron 集成（P2 阶段）

```js
// electron/main.js（示意）
const { app, BrowserWindow, Tray } = require('electron');
const { spawn } = require('child_process');
const path = require('path');
const net = require('net');

const PORT = 18080;
let backendProcess = null;

function startBackend() {
    const binaryName = process.platform === 'win32' ? 'espulse.exe' : 'espulse';
    const binaryPath = path.join(process.resourcesPath, binaryName);
    backendProcess = spawn(binaryPath, ['--port', PORT], { stdio: 'ignore' });
}

function waitForBackend(callback) {
    const check = () => {
        const client = net.connect(PORT, '127.0.0.1', () => {
            client.destroy();
            callback();
        });
        client.on('error', () => setTimeout(check, 200));
    };
    check();
}

app.whenReady().then(() => {
    startBackend();
    waitForBackend(() => {
        const win = new BrowserWindow({ width: 1440, height: 900 });
        win.loadURL(`http://127.0.0.1:${PORT}`);
    });
});

app.on('before-quit', () => {
    if (backendProcess) backendProcess.kill();
});
```

---

## 9. 关键技术决策记录

| 决策 | 选择 | 原因 |
|---|---|---|
| 后端语言 | Go + Gin | 单二进制部署 + Electron 内嵌零依赖，桌面端体验一流 |
| ES 请求方式 | 原生 `net/http`，不用官方 SDK | 天然多版本兼容，Dev Console 可完全透传 |
| SQLite 驱动 | `modernc.org/sqlite`（纯 Go） | 无 CGO，跨平台编译无障碍 |
| 前端内嵌 | `embed.FS` | 单二进制分发，服务器部署无需 Nginx |
| Dev Console 编辑器 | Monaco Editor | VS Code 同款，ES JSON schema 可直接集成补全 |
| 索引列表渲染 | 虚拟滚动（vue-virtual-scroller） | 解决 Cerebro 上万索引卡顿的根本问题 |
| 密码存储 | AES-GCM 加密写入 SQLite | 避免明文存储连接凭据 |