import type { EsIndex, EsShard } from '../../api/catalog'

export type ViewMode = 'index' | 'node'
export type SortKey = 'index' | 'health' | 'docs' | 'store' | 'write' | 'read'
export type SortOrder = 'asc' | 'desc'
export type HealthKey = 'green' | 'yellow' | 'red'

// 索引视角：每个节点一列，单元格 = 该索引在该节点上的分片
export interface WbNodeCol {
  name: string
  master: boolean
  heap: string
  cpu: string
  disk: string
}

export interface WbIndexRow {
  index: EsIndex
  // 与 nodeColumns 对齐；null 表示该节点上没有该索引的分片
  cells: (EsShard[] | null)[]
  unassigned: EsShard[]
}

export interface WbGroup {
  prefix: string
  rows: WbIndexRow[]
  worst: HealthKey
  pri: number
  rep: number
  docs: number
  store: number
}

// 节点视角：行=节点，列=索引（分页）
export interface WbIndexCol {
  name: string
  health: string
}

export interface WbNodeRow {
  name: string
  role: string
  master: boolean
  heap: string
  cpu: string
  disk: string
  load1: string
  load5: string
  load15: string
  shards: number | null
  // 与 indexColumns 对齐；null 表示该节点上没有该索引的分片
  cells: (EsShard[] | null)[]
}
