# ESPulse — Product Requirements Document

> 版本：v0.2 | 状态：草稿 | 最后更新：2026-03

---

## 1. 产品定义

### 一句话描述
ESPulse 是一个面向运维和开发工程师的 Elasticsearch 集群管理工具，整合 Kibana Dev Tools 的命令能力与 Cerebro 的集群可视化能力，支持同时管理多个版本的 ES 集群。

### 解决的核心痛点

| 现有工具 | 痛点 |
|---|---|
| Kibana | 功能臃肿；版本绑定，多集群需要多个地址；分片视角单一（节点维度） |
| Cerebro | 已停止维护；新版 OS 无法安装依赖 JDK；通用命令执行体验差；上万索引时性能极差 |

### 目标用户
- 主要：负责维护多个 ES 集群的运维/后端工程师
- 次要：需要频繁调试 ES 查询的开发工程师

---

## 2. 技术栈

| 层 | 技术 |
|---|---|
| 前端 | Vue 3 + Naive UI + TypeScript |
| 后端 | Go + Gin |
| 本地存储 | SQLite（`modernc.org/sqlite`，纯 Go 实现，无 CGO 依赖） |
| 桌面端 | Electron（内嵌 Go 二进制，零外部依赖） |
| 服务器部署 | 单二进制 + systemd 服务，前端静态资源通过 `embed.FS` 内嵌 |

### 为什么选 Go

- **单二进制部署**：编译产物是一个可执行文件，内嵌前端静态资源，服务器部署只需上传一个文件
- **Electron 内嵌无痛**：Go 二进制直接打包进 Electron，用户无需安装任何运行时，双击即用
- **跨平台交叉编译**：一台机器可编译出 Windows / macOS / Linux 三个平台的产物
- **无 CGO**：使用 `modernc.org/sqlite`（纯 Go SQLite），关闭 CGO，跨平台编译无障碍

### 部署模式说明

**服务器模式**：单个 Go 二进制，前端静态文件通过 `embed.FS` 内嵌其中。注册为 systemd 服务后台运行，局域网内所有人通过浏览器访问。

**桌面模式（Electron）**：Go 二进制放在 Electron 应用目录内，Electron 主进程启动时通过 `child_process.spawn` 拉起 Go 进程并监听本地端口，WebView 加载 `http://localhost:{port}`。最终产物是标准 Electron 安装包（`.exe` / `.dmg` / `.AppImage`）。

---

## 3. 功能清单与优先级

### P0 — 核心功能（MVP 必须包含）

#### 3.1 多集群连接管理
- 页面上可添加、编辑、删除 ES 集群连接
- 连接信息持久化存储于 SQLite（hosts、认证信息、备注、颜色标签）
- 支持任意 ES 版本（通过直接调用 REST API，不绑定特定客户端版本）
- 连接健康检测，显示集群状态（green / yellow / red / unreachable）
- 顶部全局切换当前活动集群

#### 3.2 Dev Console（命令执行）
- Monaco Editor 集成，语法高亮
- ES REST API 自动补全（基于 ES JSON schema，覆盖 6.x / 7.x / 8.x）
- 请求执行（GET / POST / PUT / DELETE / HEAD）
- 响应结果格式化展示（JSON 折叠/展开）
- 请求历史记录（持久化最近 200 条）
- 支持多 Tab 并行编辑

#### 3.3 索引列表
- 展示所有索引：名称、文档数、存储大小、主分片数、副本数、状态
- 支持名称搜索、状态筛选
- 虚拟滚动，支持上万索引流畅渲染
- 点击行展开查看 mapping / settings / aliases

#### 3.4 Shard 活动监控（现有功能整合）
- 实时展示正在初始化 / 迁移的 shard（进度、源节点、目标节点）
- 集群无 shard 活动时显示"集群稳定"
- 默认 5s 轮询，频率可配置

### P1 — 差异化功能

#### 3.5 Shard 分布可视化（双视角）
- 索引维度：以索引为行、节点为列的分布热力图，高亮分布不均的分片
- 节点维度：以节点为行，列出每个节点上的 shard
- 视角可切换
- 支持手动触发 reroute

#### 3.6 集群健康仪表盘
- 节点列表：角色、状态、磁盘用量、堆内存、CPU
- 指标超阈值时高亮告警
- 集群总览：文档数、存储量、活跃分片数

#### 3.7 索引模板 / Mapping 管理
- 查看 index template 列表
- 使用 Monaco Editor 查看 / 编辑 mapping
- 兼容 legacy template 和 composable template

### P2 — 体验提升

#### 3.8 快捷运维操作
- Clear cache、Force merge、Refresh
- Reindex（含进度展示）
- 快照仓库管理 + 创建 / 恢复快照

#### 3.9 命令收藏夹
- Dev Console 中保存常用请求片段
- 支持变量占位符（如 `{{index_name}}`）
- 分类管理

#### 3.10 Electron 桌面端打包
- 跨平台：Windows / macOS / Linux
- 系统托盘常驻，支持开机自动启动

### P3 — 未来探索
- 慢查询日志可视化
- Dev Console 内 AI 辅助（自然语言生成 DSL）
- 多用户权限管理（团队服务器部署场景）

---

## 4. UI 设计原则

- 深色科技感主题为主，提供浅色主题切换
- Naive UI 组件库 + 自定义主题 token
- 数据密集界面优先信息密度，而非大量留白
- 最低支持 1280px 宽度

---

## 5. 非功能需求

| 需求 | 指标 |
|---|---|
| 索引列表渲染 | 10,000+ 索引流畅滚动，首屏 < 1s |
| Shard 监控刷新 | 默认 5s，可配置 |
| ES 版本兼容 | ES 6.x / 7.x / 8.x |
| 跨平台 | Windows 10+ / macOS 12+ / Ubuntu 20.04+ |
| 服务器部署复杂度 | 上传单个二进制文件即可运行，无需安装运行时 |

---

## 6. 不做什么

- 不做日志查看（Kibana Logs / Discover）
- 不做 APM / Metrics 监控
- 不做 Dashboard / 可视化图表
- 不做 ES 集群的安装与部署