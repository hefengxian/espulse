<script setup lang="ts">
import { computed } from 'vue'
import { NConfigProvider, NMessageProvider, darkTheme } from 'naive-ui'
import { useTheme } from './composables/useTheme'

/**
 * Global theme configuration for Naive UI
 * This ensures consistency with our UnoCSS theme
 * Note: Naive UI's 'seemly' library needs real color strings (like #hex or rgb)
 * to perform internal color calculations (hover, pressed, etc.).
 */
const { isDark } = useTheme()

const themeOverrides = computed(() => ({
  common: {
    primaryColor: isDark.value ? '#5b6cf8' : '#3b82f6',
    primaryColorHover: isDark.value ? '#6b7cff' : '#2563eb',
    primaryColorPressed: isDark.value ? '#4f5ef0' : '#1d4ed8',
    primaryColorSuppl: isDark.value ? '#5b6cf8' : '#3b82f6',
    borderRadius: '8px',
  },
  Card: {
    borderRadius: '12px',
  },
}))
</script>

<template>
  <n-config-provider :theme="isDark ? darkTheme : null" :theme-overrides="themeOverrides">
    <n-message-provider>
        <!-- Router View for Layouts and Pages -->
        <router-view />
    </n-message-provider>
  </n-config-provider>
</template>

<style>
/* Global resets or fonts if needed */
body {
  margin: 0;
  padding: 0;
  background-color: var(--esp-bg);
}

#app {
  height: 100vh;
}
</style>
