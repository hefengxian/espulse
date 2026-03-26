# TASKS — ESPulse 任务看板

> 状态：进行中 | 当前里程碑：Phase 0 - 项目初始化

---

## 🎯 当前目标
实现 ES 代理 (/api/proxy) 通用请求转发。

---

## 🛠 正在进行 (In Progress)
- [ ] **后端：ES 代理**：实现 `/api/proxy` 通用请求转发
- [ ] **前端：基础布局**：侧边栏、顶部集群选择器
- [ ] **前端：Dev Console**：Monaco Editor 集成与基础请求发送

### Phase 2: 索引与监控 (P1 功能)
- [ ] **后端：索引列表 API**：获取 ES 索引元数据
- [ ] **前端：索引管理页面**：实现虚拟滚动展示万级索引
- [ ] **后端：Shard 监控**：实现实时 Shard 活动查询
- [ ] **前端：Shard 分布图**：可视化热力图展示

### Phase 3: 桌面端与打包 (P2 功能)
- [ ] **Electron 集成**：主进程启动 Go 后端逻辑
- [ ] **自动化构建**：编写 `build.sh` 实现一键打包

---

## ✅ 已完成 (Done)
- [x] 完成 PRD 与 Architecture 架构设计文档
- [x] 完善项目文档体系 (`PRD.md`, `Architecture.md`, `TASKS.md`, `CHANGELOG.md`)
- [x] 按照架构文档搭建后端 Go 目录结构
- [x] 初始化前端 Vue 3 (Vite + TS + Naive UI) 基础结构
- [x] 配置 Air 热重载、Git 忽略文件 (.gitignore) 及空目录保护 (.gitkeep)
- [x] **数据库与模型**：初始化 SQLite 数据库及 `clusters` 表
- [x] **后端：集群管理 API**：实现集群的 CRUD 接口
