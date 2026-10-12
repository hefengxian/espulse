<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { storeToRefs } from 'pinia'
import { useClusterStore } from '../stores/cluster'
import { useGlobalRefresh } from '../composables/useGlobalRefresh'
import { useTheme } from '../composables/useTheme'
import { formatAge, isStale } from '../utils/freshness'
import ClusterSwitcher from '../components/ClusterSwitcher.vue'

const router = useRouter()
const route = useRoute()
const clusterStore = useClusterStore()
const { loaded } = storeToRefs(clusterStore)
const { isDark, toggleTheme } = useTheme()

const { handler, refreshing, updatedAt, triggerRefresh } = useGlobalRefresh()

// 活动集群由 URL 决定（Hub 本身没有 :id，落在这里时切换器显示「全部集群」）
const clusterId = computed(() => route.params.id as string | undefined)
const currentCluster = computed(() => clusterStore.clusterById(clusterId.value))

// 数据新鲜度常驻顶栏（见 PRD §2.4「像行情看板」）
const now = ref(Date.now())
let ticker: number | undefined

const freshnessText = computed(() => formatAge(updatedAt.value, now.value))
const stale = computed(() => isStale(updatedAt.value, now.value, 15000))

// 模块 Tab 已随旧版模块页一并取消：集群只有一个落地页（`/workbench/:id`），
// Hub 只负责「管理 + 浏览」（见 PRD §2.5）

onMounted(async () => {
  ticker = window.setInterval(() => { now.value = Date.now() }, 5000)

  try {
    await clusterStore.fetchClusters()
  } catch {
    // 集群列表加载失败由 Cluster Hub 自行提示
  }
})

// 集群已被删除、URL 又是旧链接（历史 / 书签）时统一收敛回列表，
// 避免渲染出一套点进去是空白的视图
watch([loaded, clusterId, () => clusterStore.clusters.length], () => {
  if (loaded.value && clusterId.value && !currentCluster.value) router.replace('/')
})

onBeforeUnmount(() => {
  if (ticker) window.clearInterval(ticker)
})
</script>

<template>
  <div class="esp-app-container h-screen flex flex-col overflow-hidden font-sans bg-bg text-text antialiased">
    <header class="esp-header h-13 bg-bg-2 flex items-center gap-2.5 px-4 flex-shrink-0">
      <div class="w-7 h-7 flex-shrink-0 bg-gradient-to-br from-accent to-purple-600 rounded-7px flex items-center justify-center shadow-[0_0_16px_var(--esp-accent-glow)]">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <circle cx="7" cy="7" r="3" fill="white" opacity=".9" />
          <circle cx="7" cy="7" r="6" stroke="white" stroke-width="1" opacity=".4" />
        </svg>
      </div>

      <!-- 层 1：我在看哪个集群（Hub 只是管理页，集群本身落在工作台） -->
      <ClusterSwitcher />

      <div class="ml-auto flex items-center gap-2.5 flex-shrink-0">
        <span v-if="clusterId" class="text-11.5px" :class="stale ? 'color-yellow' : 'text-text-3'">
          {{ freshnessText }}<template v-if="stale"> · 正在刷新…</template>
        </span>
        <button class="btn-icon" :disabled="!handler" title="刷新当前集群" @click="triggerRefresh">
          <div class="w-3.75 h-3.75" :class="refreshing ? 'i-lucide-loader-circle animate-spin' : 'i-lucide-refresh-cw'"></div>
        </button>
        <button class="btn-icon" :title="isDark ? '切换到浅色' : '切换到深色'" @click="toggleTheme">
          <div class="w-3.75 h-3.75" :class="isDark ? 'i-lucide-sun' : 'i-lucide-moon'"></div>
        </button>
      </div>
    </header>

    <div id="content" class="esp-content flex-1 overflow-y-auto overflow-x-hidden bg-bg">
      <router-view />
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
