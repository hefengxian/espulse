# ESPulse — 架构设计文档

> 版本：v0.2 | 对应 PRD：v0.2

---

## 1. 整体架构

```
┌─────────────────────────────────────────────────────┐
│                    前端 (Vue 3)                       │
│  ┌──────────┐ ┌─────────────┐ ┌──────────────────┐  │
│  │ 集群管理  │ │ Dev Console │ │  Shard 可视化     │  │
│  └──────────┘ └─────────────┘ └──────────────────┘  │
│              Naive UI + Pinia + Vue Router            │
└────────────────────┬────────────────────────────────┘
                     │ HTTP (REST API)
┌────────────────────▼────────────────────────────────┐
│                后端 (Go + Gin)                        │
│                                                       │
│  /api/clusters/*   集群 CRUD + 健康检测               │
│  /api/proxy/*      ES 请求透明转发（Dev Console 核心）│
│  /api/indices/*    索引列表（服务端过滤/分页）         │
│  /api/shards/*     Shard 状态与分布                   │
│  /api/nodes/*      节点信息                           │
│  /*                静态资源（embed.FS 内嵌前端）       │
│                                                       │
│  ┌─────────────────────────────────────────────┐     │
│  │           SQLite (modernc.org/sqlite)        │     │
│  │   clusters / snippets / console_history      │     │
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
│   ├── config/
│   │   └── config.go            # 从环境变量 / 配置文件读取（端口、数据目录等）
│   │
│   ├── database/
│   │   ├── db.go                # SQLite 连接初始化（modernc.org/sqlite + database/sql）
│   │   └── migrate.go           # 建表 / 版本迁移
│   │
│   ├── models/
│   │   ├── cluster.go           # Cluster 结构体 + CRUD 方法
│   │   ├── snippet.go           # Snippet 结构体 + CRUD 方法
│   │   └── history.go           # ConsoleHistory 结构体 + CRUD 方法
│   │
│   ├── handlers/
│   │   ├── clusters.go          # GET/POST/PUT/DELETE /api/clusters
│   │   ├── proxy.go             # POST /api/proxy/:cluster_id  ES 请求转发
│   │   ├── indices.go           # GET /api/clusters/:id/indices
│   │   ├── shards.go            # GET /api/clusters/:id/shards
│   │   └── nodes.go             # GET /api/clusters/:id/nodes
│   │
│   ├── es/
│   │   ├── client.go            # ES 客户端池（cluster_id → *http.Client + baseURL）
│   │   └── request.go           # 封装向 ES 发送 HTTP 请求的通用方法
│   │
│   └── router/
│       └── router.go            # Gin 路由注册 + 静态资源 embed 挂载
│
├── frontend/                    # Vue 3 前端（独立开发，构建产物内嵌进 Go）
│   ├── src/
│   │   ├── main.ts
│   │   ├── router/index.ts
│   │   ├── stores/
│   │   │   ├── cluster.ts       # 当前活动集群
│   │   │   └── console.ts       # Dev Console Tab / 历史状态
│   │   ├── views/
│   │   │   ├── Dashboard.vue
│   │   │   ├── Console.vue
│   │   │   ├── Indices.vue
│   │   │   ├── Shards.vue
│   │   │   └── Settings.vue
│   │   ├── components/
│   │   │   ├── ClusterSelector.vue
│   │   │   ├── ShardHeatmap.vue
│   │   │   └── MonacoEditor.vue
│   │   └── api/
│   │       ├── clusters.ts
│   │       ├── proxy.ts
│   │       ├── indices.ts
│   │       └── shards.ts
│   ├── package.json
│   └── vite.config.ts           # 构建输出到 ../static/dist/
│
├── static/
│   └── dist/                    # Vite 构建产物（由 embed.FS 内嵌）
│
├── electron/                    # 桌面端（P2 阶段）
│   ├── main.js                  # 主进程：拉起 Go 子进程 + 创建 BrowserWindow
│   ├── preload.js
│   └── package.json
│
├── scripts/
│   ├── build.sh                 # 一键构建：前端 → Go 编译 → 输出二进制
│   └── build-all.sh             # 交叉编译三平台产物
│
└── espulse.service              # systemd 服务模板
```

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
```

---

## 4. API 设计

### 集群管理
```
GET    /api/clusters                    列出所有集群（含实时健康状态）
POST   /api/clusters                    添加集群（保存前做连通性测试）
GET    /api/clusters/:id                获取单个集群详情
PUT    /api/clusters/:id                更新集群配置
DELETE /api/clusters/:id                删除集群
POST   /api/clusters/:id/test           单独测试连接（不保存）
```

### ES 请求代理（Dev Console 核心）
```
POST   /api/proxy/:cluster_id

Request body:
{
  "method": "GET",
  "path":   "/_cat/indices?v&format=json",
  "body":   {}
}

Response:
{
  "status":      200,
  "duration_ms": 42,
  "body":        { ...ES 原始响应... }
}
```
后端收到请求后，从连接池取出对应集群的 HTTP 客户端，原样转发请求并返回 ES 响应。**不对请求内容做任何解析或封装**，保持与 ES 的完全兼容。

### 集群数据接口
```
GET    /api/clusters/:id/nodes
GET    /api/clusters/:id/indices?search=&status=&page=&page_size=
GET    /api/clusters/:id/shards
GET    /api/clusters/:id/shards/active   仅返回 INITIALIZING / RELOCATING 状态的分片
```

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