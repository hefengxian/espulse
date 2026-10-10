// 展示层的数字 / 字节 / 速率格式化。工作台的指标条、网格、状态栏共用同一套口径。

export function fmtInt(value: number | string | undefined | null): string {
  const n = Number(value)
  return Number.isFinite(n) ? n.toLocaleString('en-US') : '—'
}

export function fmtBytes(bytes: number | string | undefined | null): string {
  const value = Number(bytes)
  if (!Number.isFinite(value)) return '—'
  if (value === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let v = value
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

// 速率是后端就地差分出来的派生值；样本不足时字段缺省，显示 "—"。
// 超过 1000 用 k 简写以缩短长度，口径与 Indices 栏目保持一致。
export function fmtRate(value: number | undefined | null): string {
  if (value === undefined || value === null || !Number.isFinite(value)) return '—'
  if (value === 0) return '0'
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}k`
  if (value >= 10) return String(Math.round(value))
  return value.toFixed(1)
}

// 资源使用率着色：超过 high 标红，超过 mid 标黄（用于 heap / cpu / disk）
export function usageClass(value: string | number | undefined, high: number, mid: number): '' | 'hi' | 'cr' {
  const n = Number(value)
  if (!Number.isFinite(n)) return ''
  if (n >= high) return 'cr'
  if (n >= mid) return 'hi'
  return ''
}
