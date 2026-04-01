<script setup lang="ts">
import { h } from 'vue'
import { NDataTable, NTag, NButton } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

interface IndexData {
  name: string
  health: 'green' | 'yellow' | 'red'
  docs: string
  size: string
  shards: string
}

const columns: DataTableColumns<IndexData> = [
  {
    title: 'Name',
    key: 'name',
    render(row) {
      return h('span', { class: 'font-mono text-12.5px' }, row.name)
    }
  },
  {
    title: 'Health',
    key: 'health',
    render(row) {
      const colorMap = {
        green: 'var(--esp-green)',
        yellow: 'var(--esp-yellow)',
        red: 'var(--esp-red)'
      }
      return h('div', { class: 'flex items-center gap-1.25 text-12px font-600 font-mono', style: { color: colorMap[row.health] } }, [
        h('span', { class: 'w-1.25 h-1.25 rounded-full bg-current' }),
        row.health
      ])
    }
  },
  {
    title: 'Docs',
    key: 'docs'
  },
  {
    title: 'Size',
    key: 'size'
  },
  {
    title: 'Shards',
    key: 'shards'
  }
]

const data: IndexData[] = [
  { name: 'logs-prod-2024.01', health: 'green', docs: '1.2B', size: '48 GB', shards: '5 / 1' },
  { name: 'logs-prod-2024.02', health: 'green', docs: '850M', size: '32 GB', shards: '5 / 1' },
  { name: 'metrics-system-2024.03', health: 'yellow', docs: '2.4B', size: '112 GB', shards: '5 / 1' },
  { name: 'app-events-prod', health: 'green', docs: '120M', size: '12 GB', shards: '3 / 1' },
  { name: 'search-index-v2', health: 'green', docs: '45M', size: '4 GB', shards: '1 / 1' },
]
</script>

<template>
  <div class="animate-in fade-in duration-300">
    <div class="text-18px font-600 tracking--0.4px mb-1">Overview</div>
    <div class="text-13px text-text-2 mb-6">prod-us-east-1 &nbsp;·&nbsp; Elasticsearch 8.12.1 &nbsp;·&nbsp; 3 nodes &nbsp;·&nbsp; Last updated just now</div>

    <!-- Stats row -->
    <div class="grid grid-cols-4 gap-3 mb-5">
      <div class="bg-bg-2 border border-border rounded-10px p-4 px-4.5 flex flex-col gap-1.5 transition-all hover:border-border-2 shadow-sm">
        <div class="text-11.5px text-text-3 font-500 tracking-0.03em flex items-center gap-1.5">
          <span class="w-1.5 h-1.5 rounded-full bg-green"></span>Cluster Health
        </div>
        <div class="text-18px font-600 font-sans mt-0.5 text-green">GREEN</div>
        <div class="text-11.5px text-text-3">All shards assigned</div>
      </div>
      <div class="bg-bg-2 border border-border rounded-10px p-4 px-4.5 flex flex-col gap-1.5 transition-all hover:border-border-2 shadow-sm">
        <div class="text-11.5px text-text-3 font-500 tracking-0.03em flex items-center gap-1.5">Total Indices</div>
        <div class="text-26px font-600 tracking--0.8px leading-none font-mono">24</div>
        <div class="text-11.5px text-green">+2 this week</div>
      </div>
      <div class="bg-bg-2 border border-border rounded-10px p-4 px-4.5 flex flex-col gap-1.5 transition-all hover:border-border-2 shadow-sm">
        <div class="text-11.5px text-text-3 font-500 tracking-0.03em flex items-center gap-1.5">
          <span class="w-1.5 h-1.5 rounded-full bg-yellow"></span>Unassigned Shards
        </div>
        <div class="text-26px font-600 tracking--0.8px leading-none font-mono text-yellow">3</div>
        <div class="text-11.5px text-yellow">Needs attention</div>
      </div>
      <div class="bg-bg-2 border border-border rounded-10px p-4 px-4.5 flex flex-col gap-1.5 transition-all hover:border-border-2 shadow-sm">
        <div class="text-11.5px text-text-3 font-500 tracking-0.03em flex items-center gap-1.5">Total Documents</div>
        <div class="text-26px font-600 tracking--0.8px leading-none font-mono">4.2B</div>
        <div class="text-11.5px text-text-3">Across all indices</div>
      </div>
    </div>

    <!-- Two col -->
    <div class="grid grid-cols-[1fr_340px] gap-3 mb-5">
      <!-- Indices table -->
      <div class="bg-bg-2 border border-border rounded-10px overflow-hidden shadow-sm">
        <div class="flex items-center justify-between p-3.5 px-4.5 border-b border-border bg-bg-2">
          <span class="text-13.5px font-600 tracking--0.2px">Indices</span>
          <div class="flex gap-1.5">
            <n-button size="tiny" quaternary border-radius="6px">Filter</n-button>
            <n-button size="tiny" quaternary border-radius="6px">View all</n-button>
          </div>
        </div>
        <n-data-table
          :columns="columns"
          :data="data"
          :bordered="false"
          :single-line="false"
          size="small"
        />
      </div>

      <!-- Shard map -->
      <div class="bg-bg-2 border border-border rounded-10px flex flex-col overflow-hidden">
        <div class="flex items-center justify-between p-3.5 px-4.5 border-b border-border">
          <span class="text-13.5px font-600 tracking--0.2px">Shard Map</span>
          <button class="text-12px p-1 px-2.5 rounded-6px border border-border bg-transparent text-text-2 cursor-pointer transition-all hover:bg-bg-3 hover:text-text">Details</button>
        </div>
        <div class="flex flex-wrap gap-0.75 p-4 px-4.5">
          <div v-for="i in 80" :key="i" 
            class="w-3.5 h-3.5 rounded-3px cursor-default transition-transform hover:scale-125"
            :class="[
              i < 70 ? 'bg-green opacity-85' : 
              i < 75 ? 'bg-yellow opacity-85' : 
              'bg-bg-4 border border-border'
            ]"
          ></div>
        </div>
        <div class="flex gap-3.5 px-4.5 pb-3.5 border-t border-border pt-3">
          <span class="flex items-center gap-1.25 text-11.5px text-text-3"><span class="w-2 h-2 rounded-2px bg-green"></span>Assigned</span>
          <span class="flex items-center gap-1.25 text-11.5px text-text-3"><span class="w-2 h-2 rounded-2px bg-yellow"></span>Relocating</span>
          <span class="flex items-center gap-1.25 text-11.5px text-text-3"><span class="w-2 h-2 rounded-2px bg-bg-4 border border-border"></span>Unassigned</span>
        </div>
      </div>
    </div>

    <!-- Nodes -->
    <div class="text-13.5px font-600 tracking--0.2px mb-3">Nodes</div>
    <div class="grid grid-cols-3 gap-3 mb-5">
      <div v-for="i in 3" :key="i" class="bg-bg-2 border border-border rounded-10px p-3.5 px-4 transition-all hover:border-border-2">
        <div class="flex items-center justify-between mb-3">
          <span class="text-12.5px font-600 font-mono">es-node-0{{ i }}</span>
          <span class="text-10.5px px-1.75 py-0.5 rounded-full bg-bg-3 border border-border text-text-3 font-500">master · data</span>
        </div>
        <div v-for="label in ['CPU', 'Heap', 'Disk']" :key="label">
          <div class="text-11px text-text-3 flex justify-between mb-1">
            <span>{{ label }}</span>
            <span class="font-mono">42%</span>
          </div>
          <div class="h-1 bg-bg-3 rounded-full mb-2.5 overflow-hidden">
            <div class="h-full rounded-full bg-accent" style="width: 42%"></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
