<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useClusterStore } from '../stores/cluster'

const router = useRouter()
const route = useRoute()
const clusterStore = useClusterStore()

const isCollapsed = ref(false)
const isDark = ref(true)

// 活动集群由 URL 决定（/cluster/:id/...），不再使用隐式的全局选择状态
const clusterId = computed(() => route.params.id as string | undefined)
const currentCluster = computed(() => clusterStore.clusterById(clusterId.value))

interface NavItem {
  id: string
  label: string
  icon: string
  path: string
}

// 集群相关入口只在进入某个集群后出现
const navItems = computed<NavItem[]>(() => {
  const items: NavItem[] = []
  const id = clusterId.value

  if (id) {
    items.push({ id: 'overview', label: 'Overview', icon: 'i-lucide-layout-grid', path: `/cluster/${id}/overview` })
    items.push({ id: 'indices', label: 'Indices', icon: 'i-lucide-list', path: `/cluster/${id}/indices` })
    items.push({ id: 'shards', label: 'Shards', icon: 'i-lucide-grid-3x3', path: `/cluster/${id}/shards` })
    items.push({ id: 'console', label: 'Dev Console', icon: 'i-lucide-terminal', path: `/cluster/${id}/console` })
  }

  items.push({ id: 'clusters', label: 'Clusters', icon: 'i-lucide-layers', path: '/' })
  return items
})

const toggleSidebar = () => {
  isCollapsed.value = !isCollapsed.value
}

const toggleTheme = () => {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('light', !isDark.value)
  document.documentElement.classList.toggle('dark', isDark.value)
}

const setActive = (path: string) => {
  router.push(path)
}

onMounted(async () => {
  document.documentElement.classList.add('dark')
  try {
    await clusterStore.fetchClusters()
  } catch {
    // 集群列表加载失败由 Cluster Hub 自行提示
  }
})
</script>

<template>
  <div class="esp-app-container h-screen flex overflow-hidden font-sans bg-bg text-text antialiased">
    <!-- Sidebar -->
    <aside
      id="sidebar"
      :class="[{ 'w-56 min-w-56': !isCollapsed, 'w-14 min-w-14': isCollapsed }, 'esp-sidebar']"
      class="bg-bg-2 border-r border-border flex flex-col transition-all duration-220 z-10 flex-shrink-0 overflow-hidden"
    >
      <div class="esp-sidebar-header h-13 flex items-center gap-2.5 px-3.5 border-b border-border flex-shrink-0">
        <div class="w-7 h-7 flex-shrink-0 bg-gradient-to-br from-accent to-purple-600 rounded-7px flex items-center justify-center shadow-[0_0_16px_var(--esp-accent-glow)]">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
            <circle cx="7" cy="7" r="3" fill="white" opacity=".9" />
            <circle cx="7" cy="7" r="6" stroke="white" stroke-width="1" opacity=".4" />
          </svg>
        </div>
        <span v-if="!isCollapsed" class="font-600 text-15px tracking--0.3px whitespace-nowrap overflow-hidden transition-opacity duration-180">
          ESPulse
        </span>
      </div>

      <div class="esp-sidebar-nav p-2 pt-0 flex-1 overflow-hidden">
        <div v-if="!isCollapsed" class="text-10.5px font-500 tracking-0.08em text-text-3 uppercase p-2 pb-1 whitespace-nowrap overflow-hidden transition-opacity">
          Navigation
        </div>
        <div
          v-for="item in navItems"
          :key="item.id"
          class="flex items-center gap-2.5 p-1.75 px-2 rounded-6px cursor-pointer text-text-2 transition-all duration-120 whitespace-nowrap relative hover:bg-bg-3 hover:text-text"
          :class="{ '!bg-accent-glow !text-accent': route.path === item.path }"
          @click="setActive(item.path)"
        >
          <div :class="[item.icon, { 'text-accent': route.path === item.path }]" class="w-4 h-4 flex-shrink-0"></div>
          <span v-if="!isCollapsed" class="text-13.5px font-500 overflow-hidden transition-opacity">
            {{ item.label }}
          </span>
        </div>
      </div>

      <div class="esp-sidebar-footer p-2 border-t border-border flex-shrink-0">
        <button class="flex items-center justify-center w-full p-1.75 px-2 rounded-6px cursor-pointer text-text-3 transition-all duration-120 border-none bg-transparent hover:bg-bg-3 hover:text-text" @click="toggleSidebar">
          <div class="w-4 h-4 transition-transform duration-220 i-lucide-panel-left-close" :class="{ 'rotate-180': isCollapsed }"></div>
        </button>
      </div>
    </aside>

    <!-- Main Content Area -->
    <div id="main" class="esp-main-container flex-1 flex flex-col overflow-hidden min-w-0">
      <header class="esp-header h-13 bg-bg-2 border-b border-border flex items-center gap-2.5 px-4 flex-shrink-0">
        <div
          class="esp-header-left flex items-center gap-2 p-1.25 px-2.5 rounded-7px border border-border bg-bg cursor-pointer transition-all hover:border-border-2"
          @click="router.push('/')"
        >
          <div
            class="w-2 h-2 rounded-full flex-shrink-0"
            :style="{ backgroundColor: currentCluster ? `var(--esp-${currentCluster.color || 'green'})` : 'var(--esp-text-3)' }"
          ></div>
          <span class="text-13px font-500 flex-1">{{ currentCluster?.name || '集群' }}</span>
        </div>

        <div class="esp-header-right flex items-center gap-1.5 ml-auto">
          <button class="w-8 h-8 rounded-7px border border-border bg-transparent text-text-2 flex items-center justify-center cursor-pointer transition-all hover:bg-bg-3 hover:text-text hover:border-border-2" @click="toggleTheme">
            <div class="w-3.75 h-3.75" :class="isDark ? 'i-lucide-sun' : 'i-lucide-moon'"></div>
          </button>
        </div>
      </header>

      <div id="content" class="esp-content flex-1 overflow-y-auto overflow-x-hidden p-6 bg-bg">
        <router-view />
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Any specific styles that UnoCSS doesn't cover easily */
::-webkit-scrollbar {
  width: 5px;
  height: 5px;
}
::-webkit-scrollbar-track {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  background: var(--border-2);
  border-radius: 999px;
}
</style>
