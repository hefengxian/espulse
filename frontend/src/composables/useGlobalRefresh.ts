import { ref } from 'vue'

type RefreshHandler = () => Promise<void>

// 刷新是全局单入口（见 PRD §2.5）：顶栏那个按钮作用于「当前集群的全部数据」，
// 而不是当前模块。所以由当前激活的模块把刷新函数注册进来，顶栏只管调用。
const handler = ref<RefreshHandler | null>(null)
const refreshing = ref(false)
const updatedAt = ref('')

export function registerRefresh(fn: RefreshHandler) {
  handler.value = fn
}

export function unregisterRefresh(fn: RefreshHandler) {
  if (handler.value === fn) handler.value = null
}

export function setUpdatedAt(value: string | undefined | null) {
  updatedAt.value = value ?? ''
}

export async function triggerRefresh() {
  const fn = handler.value
  if (!fn || refreshing.value) return
  refreshing.value = true
  try {
    await fn()
  } finally {
    refreshing.value = false
  }
}

export function useGlobalRefresh() {
  return {
    handler,
    refreshing,
    updatedAt,
    registerRefresh,
    unregisterRefresh,
    setUpdatedAt,
    triggerRefresh,
  }
}
