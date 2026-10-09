import { reactive, toRefs, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

export type FilterValue = string | number | boolean

const STORAGE_KEY = 'espulse:view-filters'

type Stored = Record<string, Record<string, FilterValue>>

function readStore(): Stored {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as Stored) : {}
  } catch {
    return {}
  }
}

function writeStore(state: Stored) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    // 写不进去（隐私模式等）时退化为仅本次会话有效
  }
}

function coerce(raw: unknown, fallback: FilterValue): FilterValue {
  if (typeof fallback === 'number') {
    const n = Number(raw)
    return Number.isFinite(n) ? n : fallback
  }
  if (typeof fallback === 'boolean') return raw === '1' || raw === 'true' || raw === true
  return typeof raw === 'string' ? raw : fallback
}

// 与默认值相同的项不写进 URL，避免地址栏被默认值塞满
function encode(value: FilterValue, fallback: FilterValue): string | undefined {
  if (value === fallback) return undefined
  if (typeof value === 'boolean') return value ? '1' : '0'
  const text = String(value)
  return text === '' ? undefined : text
}

/**
 * 模块内的过滤 / 排序 / 分页状态。
 *
 * 解析优先级：URL query → 该集群上次的状态 → 默认值（见 PRD §2.5）。
 * 状态挂在「集群」上而不是「模块」上：同一集群内各模块共享同一份条件，跨集群互相隔离。
 */
export function useViewFilters<T extends Record<string, FilterValue>>(
  clusterId: Ref<string | undefined>,
  defaults: T,
) {
  const route = useRoute()
  const router = useRouter()
  const state = reactive({ ...defaults }) as Record<string, FilterValue>
  const keys = Object.keys(defaults)

  function hydrate(id: string | undefined, useQuery = true) {
    const stored = (id && readStore()[id]) || {}
    for (const key of keys) {
      const fromQuery = useQuery ? route.query[key] : undefined
      if (fromQuery !== undefined) {
        state[key] = coerce(Array.isArray(fromQuery) ? fromQuery[0] : fromQuery, defaults[key])
      } else if (stored[key] !== undefined) {
        state[key] = coerce(stored[key], defaults[key])
      } else {
        state[key] = defaults[key]
      }
    }
  }

  hydrate(clusterId.value)

  watch(clusterId, (id) => {
    // 切集群时不继承 URL / 上一个集群的条件（状态挂在集群上）：
    // 先同步清掉地址栏里的条件，再按目标集群自己的记录恢复
    const query = { ...route.query }
    let changed = false
    for (const key of keys) {
      if (query[key] !== undefined) {
        delete query[key]
        changed = true
      }
    }
    if (changed) router.replace({ query })
    hydrate(id, false)
  })

  let syncing = false
  watch(
    () => ({ ...state }),
    async (next) => {
      if (syncing) return
      const id = clusterId.value
      if (id) {
        const all = readStore()
        all[id] = { ...(all[id] ?? {}), ...next }
        writeStore(all)
      }
      const query = { ...route.query }
      for (const key of keys) {
        const encoded = encode(next[key], defaults[key])
        if (encoded === undefined) delete query[key]
        else query[key] = encoded
      }
      syncing = true
      try {
        await router.replace({ query })
      } finally {
        syncing = false
      }
    },
  )

  // 深链入口：重新回到本模块时，URL 上的参数优先（如总览的问题清单跳到分片页）
  function applyQuery() {
    if (keys.some(key => route.query[key] !== undefined)) hydrate(clusterId.value)
  }

  return { ...toRefs(state), applyQuery } as { [K in keyof T]: Ref<T[K]> } & { applyQuery: () => void }
}
