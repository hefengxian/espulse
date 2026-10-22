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
  // 组内无副本的索引数：组头汇总值不会暴露「个别索引 0 副本」，单独计数用于告警
  repZero: number
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
  // 节点地址 ip:port：角色在节点视角的各列里已能看出来，位置留给更实用的连接信息
  addr: string
  master: boolean
  heap: string
  heapMax: string
  cpu: string
  disk: string
  load1: string
  load5: string
  load15: string
  shards: number | null
  // 与 indexColumns 对齐；null 表示该节点上没有该索引的分片
  cells: (EsShard[] | null)[]
}
