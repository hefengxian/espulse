# ESPulse

ESPulse is a lightweight Elasticsearch cluster management tool designed for DevOps and backend engineers. It combines the command execution power of Kibana Dev Tools with the cluster visualization capabilities of Cerebro.

## 🚀 Key Features

- **Multi-Cluster Management**: Manage multiple ES clusters with different versions in one place.
- **Dev Console**: Monaco Editor integration with REST API auto-completion and execution.
- **Shard Visualization**: Real-time shard distribution heatmap and health monitoring.
- **Single Binary Deployment**: Built with Go + Vue 3, distributed as a zero-dependency executable.

## 🛠 Getting Started

### Backend (Go)
1. Install Go 1.26+. If your local Go is older, `GOTOOLCHAIN=auto` downloads a suitable toolchain automatically.
2. Start the server with hot reload: `go tool air`

`air` is declared in `go.mod` via the `tool` directive, so its version is pinned in `go.sum` and `go tool` resolves it directly — no `go install`, no `PATH` setup. To add another dev tool:

```sh
go get -tool github.com/example/tool@latest
go tool <tool-name>
```

### Frontend (Vue 3)
1. Navigate to directory: `cd frontend`
2. Install dependencies: `pnpm install`
3. Start dev server: `pnpm dev`

## 📂 Project Documentation

- [PRD.md](./PRD.md) - Product Requirements Document
- [Architecture.md](./Architecture.md) - Technical Architecture Design
- [TASKS.md](./TASKS.md) - Task Board & Progress
- [CHANGELOG.md](./CHANGELOG.md) - Change Log

---
*Powered by Go, Vue 3 and SQLite.*
