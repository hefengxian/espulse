// 节点 / 索引 / 分片列表：直接来自 ES 的 _cat/*?format=json，
// 键名沿用 ES 原始列名（含点号），数值列是字符串。

export interface EsShard {
  index: string
  shard: string
  prirep: string
  state: string
  node: string
  'unassigned.reason'?: string
}

export interface EsNode {
  name: string
  'node.role': string
  master: string
  'heap.percent': string
  cpu: string
  'disk.used_percent': string
}

export interface EsIndex {
  index: string
  health: string
  status: string
  pri: string
  rep: string
  'docs.count': string
  'store.size': string
}

// updated_at 是这批数据的采集时间；data 为 null 表示尚未取到数据
export interface CatalogResponse<T> {
  data: T[] | null
  updated_at: string
}

export interface IndicesQuery {
  search?: string
  health?: string
  status?: string
  sort?: string
  order?: 'asc' | 'desc'
  page?: number
  pageSize?: number
}

// 索引数量无上界，后端分页返回
export interface IndicesPage {
  data: EsIndex[] | null
  total: number
  page: number
  page_size: number
  updated_at: string
}

async function getJson<T>(url: string): Promise<T> {
  const response = await fetch(url)
  if (!response.ok) throw new Error('Request failed')
  return response.json()
}

const withRefresh = (refresh: boolean) => (refresh ? '?refresh=1' : '')

export const catalogApi = {
  nodes: (clusterId: string, refresh = false) =>
    getJson<CatalogResponse<EsNode>>(`/api/clusters/${clusterId}/nodes${withRefresh(refresh)}`),
  // 索引列表：过滤与分页都交给后端（见 PRD §10）
  indices: (clusterId: string, query: IndicesQuery = {}, refresh = false) => {
    const params = new URLSearchParams()
    if (query.search) params.set('search', query.search)
    if (query.health) params.set('health', query.health)
    if (query.status) params.set('status', query.status)
    if (query.sort) params.set('sort', query.sort)
    if (query.order) params.set('order', query.order)
    if (query.page) params.set('page', String(query.page))
    if (query.pageSize) params.set('page_size', String(query.pageSize))
    if (refresh) params.set('refresh', '1')
    const qs = params.toString()
    return getJson<IndicesPage>(`/api/clusters/${clusterId}/indices${qs ? '?' + qs : ''}`)
  },
  shards: (clusterId: string, refresh = false) =>
    getJson<CatalogResponse<EsShard>>(`/api/clusters/${clusterId}/shards${withRefresh(refresh)}`),
}

// ---------- 分片分配开关 ----------
// cluster.routing.allocation.enable 决定「允许分配哪些分片」。
// 读写都走通用 ES 代理（/api/proxy/_cluster/settings），不经后端专用接口。
export type AllocationEnable = 'all' | 'primaries' | 'new_primaries' | 'none'

export const ALLOCATION_ENABLE_KEY = 'cluster.routing.allocation.enable'

interface ClusterSettingsResponse {
  persistent?: Record<string, string>
  transient?: Record<string, string>
}

export const allocationApi = {
  // 生效值优先级：transient > persistent，都未设置时 ES 默认 all
  async get(clusterId: string): Promise<AllocationEnable> {
    const res = await fetch('/api/proxy/_cluster/settings?flat_settings=true', {
      headers: { 'X-Cluster-ID': clusterId },
    })
    if (!res.ok) throw new Error(`读取集群设置失败 (${res.status})`)
    const data: ClusterSettingsResponse = await res.json()
    const value = data.transient?.[ALLOCATION_ENABLE_KEY] ?? data.persistent?.[ALLOCATION_ENABLE_KEY]
    return (value as AllocationEnable) || 'all'
  },

  // 写入 transient（临时设置，集群重启后失效），同时清空同名 persistent，保证改动真正是临时的
  async set(clusterId: string, value: AllocationEnable): Promise<void> {
    const res = await fetch('/api/proxy/_cluster/settings', {
      method: 'PUT',
      headers: { 'X-Cluster-ID': clusterId, 'Content-Type': 'application/json' },
      body: JSON.stringify({
        transient: { [ALLOCATION_ENABLE_KEY]: value },
        // persistent: { [ALLOCATION_ENABLE_KEY]: null },
      }),
    })
    if (!res.ok) throw new Error(`写入集群设置失败 (${res.status})`)
  },
}
