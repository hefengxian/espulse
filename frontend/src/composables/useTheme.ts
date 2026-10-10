import { ref } from 'vue'

// 明暗主题地基：与「配色主题（主题色）」解耦，只负责 dark / light 两态。
// 真相源是 <html> 上的 .dark / .light 类，CSS 变量由样式层声明；
// 这里只做「读取偏好 → 应用类 → 持久化」。
export const THEME_KEY = 'espulse:theme'

export type ThemeMode = 'dark' | 'light'

const isDark = ref(true)
let initialized = false

function apply() {
  const root = document.documentElement
  root.classList.toggle('light', !isDark.value)
  root.classList.toggle('dark', isDark.value)
  try {
    localStorage.setItem(THEME_KEY, isDark.value ? 'dark' : 'light')
  } catch {
    // 存不下就退化为仅本次会话有效
  }
}

function readInitial(): ThemeMode {
  try {
    const stored = localStorage.getItem(THEME_KEY)
    if (stored === 'light' || stored === 'dark') return stored
  } catch {
    // 忽略
  }
  // 无显式偏好时跟随系统
  try {
    return window.matchMedia?.('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

// 幂等初始化：多个组件同时使用也不会重复读取 / 应用
function ensureInit() {
  if (initialized) return
  initialized = true
  isDark.value = readInitial() === 'dark'
  apply()
}

export function useTheme() {
  ensureInit()

  function setTheme(mode: ThemeMode) {
    isDark.value = mode === 'dark'
    apply()
  }

  function toggleTheme() {
    setTheme(isDark.value ? 'light' : 'dark')
  }

  return { isDark, setTheme, toggleTheme }
}
