import { watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { LOCALE_KEY, SUPPORTED_LOCALES, type AppLocale } from '../i18n'

// 语言切换地基：封装 vue-i18n 的 locale，并落 localStorage。
export function useLocale() {
  const { locale } = useI18n({ useScope: 'global' })

  watch(
    locale,
    (value) => {
      try {
        localStorage.setItem(LOCALE_KEY, value)
      } catch {
        // 存不下就退化为仅本次会话有效
      }
    },
    { immediate: true },
  )

  function setLocale(value: AppLocale) {
    locale.value = value
  }

  function toggleLocale() {
    const next = SUPPORTED_LOCALES.find((item) => item !== locale.value)
    if (next) locale.value = next
  }

  return { locale, setLocale, toggleLocale }
}
