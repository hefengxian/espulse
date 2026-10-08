// 集群总览：后端已经把 status / nodes / indices / shards 聚合好，
// updated_at 是这批数据的采集时间，前端需据此展示新鲜度（见 PRD §6.1）。

export interface OverviewHealth {
  status: string
  active_shards_percent: number
  active_shards: number
  active_primary_shards: number
  relocating_shards: number
  initializing_shards: number
  unassigned_shards: number
}

export interface OverviewCounts {
  nodes: number
  indices: number
  primary_shards: number
  total_shards: number
  docs: number
  store_bytes: number
}

export interface OverviewNode {
  name: string
  role: string
  master: string
  heap_percent: string
  cpu: string
  disk_used_percent: string
  shards?: number
}

export interface OverviewShard {
  index: string
  shard: string
  prirep: string
  state: string
  node: string
  reason?: string
}

export interface OverviewIndex {
  index: string
  health: string
  status: string
}

export interface OverviewProblems {
  unassigned_shards: OverviewShard[]
  unassigned_shards_total: number
  relocating_shards: OverviewShard[]
  relocating_shards_total: number
  unhealthy_indices: OverviewIndex[]
  unhealthy_indices_total: number
}

export interface Overview {
  reachable: boolean
  error?: string
  health: OverviewHealth
  counts: OverviewCounts
  nodes: OverviewNode[]
  problems: OverviewProblems
  nodes_loaded: boolean
  indices_loaded: boolean
  shards_loaded: boolean
  updated_at: string
}

export const overviewApi = {
  // refresh=true 时后端会绕过刷新间隔强制重采（用于页面的手动刷新）
  async get(clusterId: string, refresh = false): Promise<Overview> {
    const url = `/api/clusters/${clusterId}/overview${refresh ? '?refresh=1' : ''}`
    const response = await fetch(url)
    if (!response.ok) throw new Error('Failed to fetch overview')
    return response.json()
  },
}
