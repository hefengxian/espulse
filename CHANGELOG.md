# CHANGELOG — ESPulse 变更日志

> 记录项目的每一个足迹。

---

## [Unreleased]

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
