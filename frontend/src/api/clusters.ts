// 集群状态快照：来自后端后台采集，可能过期或采集失败，
// 因此必须同时使用 collected_at 展示数据新鲜度。
export interface ClusterStatus {
  reachable: boolean
  error?: string
  name?: string
  version?: string
  health?: string
  node_count: number
  index_count: number
  unassigned_shards: number
  collected_at: string
}

export interface Cluster {
  id: string
  name: string
  hosts: string[]
  auth_type: string
  username?: string
  password?: string
  api_key?: string
  color: string
  notes: string
  created_at?: string
  updated_at?: string
  status?: ClusterStatus
}

export interface ProbeInfo {
  name: string
  version: string
  health: string
  node_count: number
  unassigned_shards: number
}

export interface ProbeResult {
  reachable: boolean
  error?: string
  info?: ProbeInfo
}

// 保存结果：探测不通过时 saved 为 false，并带回失败原因
export interface SaveResult {
  saved: boolean
  error?: string
  cluster?: Cluster
}

export const clusterApi = {
  async list(): Promise<Cluster[]> {
    const response = await fetch('/api/clusters')
    if (!response.ok) throw new Error('Failed to fetch clusters')
    return response.json()
  },

  async get(id: string): Promise<Cluster> {
    const response = await fetch(`/api/clusters/${id}`)
    if (!response.ok) throw new Error('Failed to fetch cluster')
    return response.json()
  },

  // 保存前探测连通性：不落库，仅验证端点与凭据是否可用
  async probe(cluster: Partial<Cluster>): Promise<ProbeResult> {
    const response = await fetch('/api/clusters/probe', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cluster),
    })
    if (!response.ok) throw new Error('Failed to probe cluster')
    return response.json()
  },

  async create(cluster: Partial<Cluster>): Promise<SaveResult> {
    const response = await fetch('/api/clusters', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cluster),
    })
    if (!response.ok) throw new Error('Failed to create cluster')
    return response.json()
  },

  async update(id: string, cluster: Partial<Cluster>): Promise<SaveResult> {
    const response = await fetch(`/api/clusters/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cluster),
    })
    if (!response.ok) throw new Error('Failed to update cluster')
    return response.json()
  },

  async remove(id: string): Promise<void> {
    const response = await fetch(`/api/clusters/${id}`, { method: 'DELETE' })
    if (!response.ok) throw new Error('Failed to delete cluster')
  },

  // 触发后端立即重新采集一次
  async refresh(): Promise<void> {
    const response = await fetch('/api/clusters/refresh', { method: 'POST' })
    if (!response.ok) throw new Error('Failed to refresh clusters')
  },
}
