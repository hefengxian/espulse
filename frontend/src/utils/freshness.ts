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

// durationParts 把时长拆成「数字 + 单位」。数字单独成段是为了让界面能用定宽槽渲染它 ——
// 否则 9→10 这类位数变化会让整段文案宽一个字符，把旁边的元素一起顶走。
export function durationParts(ms: number): { value: string; unit: string } {
  const seconds = Math.round(ms / 1000)
  if (seconds < 60) return { value: `${seconds}`, unit: '秒' }
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return { value: `${minutes}`, unit: '分钟' }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return { value: `${hours}`, unit: '小时' }
  return { value: `${Math.floor(hours / 24)}`, unit: '天' }
}

// formatDuration 把一段时长渲染成最大的一位单位。
export function formatDuration(ms: number): string {
  const { value, unit } = durationParts(ms)
  return `${value} ${unit}`
}

// ageParts 是 formatAge 的可分段版本：拆成前缀 / 数字 / 单位，供需要数字定宽的界面使用。
// 未采集（时间缺失或明显非法）时返回 null，退化文案由调用方决定。
export function ageParts(
  collectedAt: string | undefined | null,
  now: number,
  prefix = '更新于',
): { prefix: string; value: string; unit: string } | null {
  const ms = ageMs(collectedAt, now)
  if (ms === null) return null
  const { value, unit } = durationParts(ms)
  return { prefix, value, unit: `${unit}前` }
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
