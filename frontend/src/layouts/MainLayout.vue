<script setup lang="ts">
import { ref, onMounted, h, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { 
  NDropdown, 
  NIcon, 
  NModal, 
  NForm, 
  NFormItem, 
  NInput, 
  NButton, 
  NSpace, 
  NSelect,
  useMessage
} from 'naive-ui'
import { clusterApi, type Cluster } from '../api/clusters'
import { useClusterStore } from '../stores/cluster'
import { storeToRefs } from 'pinia'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const clusterStore = useClusterStore()

const { clusters, currentCluster, loading } = storeToRefs(clusterStore)

const isCollapsed = ref(false)
const isDark = ref(true)

// Modal state
const showAddModal = ref(false)
const addForm = ref({
  name: '',
  hosts: '',
  auth_type: 'none',
  username: '',
  password: '',
  api_key: '',
  color: 'green',
  notes: ''
})

const fetchClusters = async () => {
  try {
    await clusterStore.fetchClusters()
  } catch (err) {
    message.error('Failed to load clusters')
  }
}

const handleClusterSelect = (key: string) => {
  if (key === 'add') {
    showAddModal.value = true
    return
  }
  
  clusterStore.selectCluster(key)
  if (currentCluster.value) {
    message.success(`Switched to ${currentCluster.value.name}`)
  }
}

const handleAddCluster = async () => {
  try {
    const newCluster = {
      ...addForm.value,
      hosts: addForm.value.hosts.split(',').map(h => h.trim())
    }
    const created = await clusterStore.addCluster(newCluster)
    showAddModal.value = false
    message.success('Cluster added successfully')
    // Reset form
    addForm.value = {
      name: '',
      hosts: '',
      auth_type: 'none',
      username: '',
      password: '',
      api_key: '',
      color: 'green',
      notes: ''
    }
  } catch (err) {
    message.error('Failed to add cluster')
  }
}

const clusterOptions = computed(() => {
  const options: any[] = clusters.value.map(c => ({
    label: c.name,
    key: c.id,
    icon: () => h('div', { 
      class: 'w-2 h-2 rounded-full',
      style: { backgroundColor: `var(--esp-${c.color || 'green'})` }
    })
  }))

  if (options.length > 0) {
    options.push({ type: 'divider', key: 'd1' })
  }

  options.push({
    label: 'Add New Cluster...',
    key: 'add',
    icon: () => h('div', { class: 'i-lucide-plus w-4 h-4' })
  })

  return options
})

const statusClass = computed(() => {
  console.log(currentCluster.value)
  if (!currentCluster.value) return 'bg-text-3 shadow-none'
  const color = currentCluster.value.color || 'green'
  console.log(color, `bg-${color} shadow-[0_0_6px_var(--esp-${color})]`)
  return `bg-${color} shadow-[0_0_6px_var(--esp-${color})]`
})

const navItems = [
  { id: 'dashboard', label: 'Dashboard', icon: 'i-lucide-layout-grid', path: '/' },
  { id: 'console', label: 'Dev Console', icon: 'i-lucide-terminal', path: '/console' },
  { id: 'indices', label: 'Indices', icon: 'i-lucide-database', path: '/indices', badge: '24' },
  { id: 'shards', label: 'Shards', icon: 'i-lucide-layers', path: '/shards', badge: '3', badgeColor: 'var(--yellow)' },
  { id: 'snapshots', label: 'Snapshots', icon: 'i-lucide-archive', path: '/snapshots' },
]

const systemItems = [
  { id: 'settings', label: 'Settings', icon: 'i-lucide-settings', path: '/settings' },
]

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

onMounted(() => {
  document.documentElement.classList.add('dark')
  fetchClusters()
})
</script>

<template>
  <div class="esp-app-container h-screen flex overflow-hidden font-sans bg-bg text-text antialiased">
    <!-- Sidebar -->
    <!-- ... sidebar content ... -->
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
          <div v-if="!isCollapsed && item.badge" class="ml-auto text-10px font-700 px-1.25 py-0.25 rounded-4px bg-bg-4 text-text-3 border border-border" :style="item.badgeColor ? { color: item.badgeColor, borderColor: item.badgeColor + '44' } : {}">
            {{ item.badge }}
          </div>
        </div>

        <div class="border-t border-border my-2"></div>
        
        <div v-if="!isCollapsed" class="text-10.5px font-500 tracking-0.08em text-text-3 uppercase p-2 pb-1 whitespace-nowrap overflow-hidden transition-opacity">
          System
        </div>
        <div
          v-for="item in systemItems"
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
        <n-dropdown :options="clusterOptions" trigger="click" placement="bottom-start" @select="handleClusterSelect">
          <div class="esp-header-left flex items-center gap-2 p-1.25 px-2.5 rounded-7px border border-border bg-bg cursor-pointer transition-all hover:border-border-2 hover:bg-bg-3">
            <div 
              class="w-2 h-2 rounded-full flex-shrink-0"
              :class="statusClass"
            ></div>
            <span class="text-13px font-500 flex-1">{{ currentCluster?.name || 'Select Cluster' }}</span>
            <div class="w-3.5 h-3.5 text-text-3 i-lucide-chevrons-up-down"></div>
          </div>
        </n-dropdown>
        
        <div 
          v-if="currentCluster"
          class="flex items-center gap-1.5 p-1.25 px-2.5 rounded-7px border text-12px font-600 font-mono"
          :class="{
            'border-green-border bg-green-bg color-green': currentCluster.color === 'green' || !currentCluster.color,
            'border-yellow-border bg-yellow-bg color-yellow': currentCluster.color === 'yellow',
            'border-red-border bg-red-bg color-red': currentCluster.color === 'red'
          }"
        >
          <span class="w-1.5 h-1.5 rounded-full bg-current animate-pulse"></span>
          {{ (currentCluster.color || 'green').toUpperCase() }}
        </div>

        <div class="esp-header-center flex-1 flex items-center gap-2 bg-bg border border-border rounded-7px px-3 h-8.5 transition-all max-w-95 focus-within:border-accent focus-within:shadow-[0_0_0_3px_var(--esp-accent-glow)]">
          <div class="w-3.5 h-3.5 text-text-3 i-lucide-search"></div>
          <input type="text" placeholder="Search indices, aliases, nodes…" class="flex-1 border-none bg-transparent text-text font-sans text-13.5px outline-none placeholder:text-text-3" />
          <span class="text-11px p-0.5 px-1.25 rounded-4px bg-bg-3 text-text-3 border border-border font-mono flex-shrink-0">⌘K</span>
        </div>

        <div class="esp-header-right flex items-center gap-1.5 ml-auto">
          <button class="w-8 h-8 rounded-7px border border-border bg-transparent text-text-2 flex items-center justify-center cursor-pointer transition-all hover:bg-bg-3 hover:text-text hover:border-border-2" @click="toggleTheme">
            <div class="w-3.75 h-3.75" :class="isDark ? 'i-lucide-sun' : 'i-lucide-moon'"></div>
          </button>
          <button class="w-8 h-8 rounded-7px border border-border bg-transparent text-text-2 flex items-center justify-center cursor-pointer transition-all hover:bg-bg-3 hover:text-text hover:border-border-2">
            <div class="w-3.75 h-3.75 i-lucide-bell"></div>
          </button>
          <div class="w-7 h-7 rounded-full bg-gradient-to-br from-accent to-purple-600 flex items-center justify-center text-11.5px font-600 text-white cursor-pointer flex-shrink-0">JD</div>
        </div>
      </header>

      <div id="content" class="esp-content flex-1 overflow-y-auto overflow-x-hidden p-6 bg-bg">
        <router-view />
      </div>
    </div>

    <!-- Add Cluster Modal -->
    <n-modal v-model:show="showAddModal" preset="card" title="Add New Cluster" class="max-w-md bg-bg-2">
      <n-form :model="addForm" label-placement="top">
        <n-form-item label="Cluster Name" path="name">
          <n-input v-model:value="addForm.name" placeholder="e.g. Production ES" />
        </n-form-item>
        <n-form-item label="Hosts (comma separated)" path="hosts">
          <n-input v-model:value="addForm.hosts" placeholder="http://localhost:9200" />
        </n-form-item>
        <n-form-item label="Authentication" path="auth_type">
          <n-select
            v-model:value="addForm.auth_type"
            :options="[
              { label: 'None', value: 'none' },
              { label: 'Basic Auth', value: 'basic' },
              { label: 'API Key', value: 'api_key' }
            ]"
          />
        </n-form-item>
        <template v-if="addForm.auth_type === 'basic'">
          <n-form-item label="Username">
            <n-input v-model:value="addForm.username" />
          </n-form-item>
          <n-form-item label="Password">
            <n-input v-model:value="addForm.password" type="password" />
          </n-form-item>
        </template>
        <template v-if="addForm.auth_type === 'api_key'">
          <n-form-item label="API Key">
            <n-input v-model:value="addForm.api_key" type="password" />
          </n-form-item>
        </template>
        <n-form-item label="Theme Color">
          <n-select
            v-model:value="addForm.color"
            :options="[
              { label: 'Green', value: 'green' },
              { label: 'Yellow', value: 'yellow' },
              { label: 'Red', value: 'red' },
              { label: 'Blue', value: 'accent' }
            ]"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showAddModal = false">Cancel</n-button>
          <n-button type="primary" @click="handleAddCluster">Add Cluster</n-button>
        </n-space>
      </template>
    </n-modal>
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
