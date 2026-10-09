// 数据新鲜度的统一表达。
//
// 快照可能来自很久以前（落库快照在进程重启、或空闲淘汰后重新进入时被复用），
// 所以必须能表达分钟 / 小时 / 天 —— 否则「更新于 86400 秒前」既读不通，
// 也掩盖了这份数据到底有多旧（见 PRD §6.3「不得把缓存当实时」）。

export const NEVER_COLLECTED = '尚未采集'

const MIN_VALID_MS = Date.parse('2000-01-01')

// ageMs 返回数据距 now 的毫秒数；时间缺失或明显非法（零值）时返回 null。
export function ageMs(collectedAt: string | undefined | null, now: number): number | null {
  if (!collectedAt) return null
  const ms = new Date(collectedAt).getTime()
  if (!Number.isFinite(ms) || ms < MIN_VALID_MS) return null
  return Math.max(0, now - ms)
}

// formatDuration 把一段时长渲染成最大的一位单位。
export function formatDuration(ms: number): string {
  const seconds = Math.round(ms / 1000)
  if (seconds < 60) return `${seconds} 秒`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} 分钟`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时`
  return `${Math.floor(hours / 24)} 天`
}

// formatAge 渲染成「更新于 3 分钟前」这类文案；prefix 可换成「采集失败于」等。
export function formatAge(collectedAt: string | undefined | null, now: number, prefix = '更新于'): string {
  const ms = ageMs(collectedAt, now)
  if (ms === null) return NEVER_COLLECTED
  return `${prefix} ${formatDuration(ms)}前`
}

// isStale 判断数据是否已经旧到该提示「正在刷新」。
export function isStale(collectedAt: string | undefined | null, now: number, staleMs: number): boolean {
  const ms = ageMs(collectedAt, now)
  return ms !== null && ms > staleMs
}
