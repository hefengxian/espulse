<script setup lang="ts">
import { NConfigProvider, NMessageProvider, darkTheme } from 'naive-ui'

/**
 * Global theme configuration for Naive UI
 * This ensures consistency with our UnoCSS theme
 * Note: Naive UI's 'seemly' library needs real color strings (like #hex or rgb)
 * to perform internal color calculations (hover, pressed, etc.).
 */
const themeOverrides = {
  common: {
    primaryColor: '#5b6cf8', // Matches --esp-accent in style.less
    primaryColorHover: '#6b7cff',
    primaryColorPressed: '#4f5ef0',
    primaryColorSuppl: '#5b6cf8',
    borderRadius: '8px',
  },
  Card: {
    borderRadius: '12px',
  },
}
</script>

<template>
  <n-config-provider :theme="darkTheme" :theme-overrides="themeOverrides">
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
  background-color: #0a0a0b;
}

#app {
  height: 100vh;
}
</style>
