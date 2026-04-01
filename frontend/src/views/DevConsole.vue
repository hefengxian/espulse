<script setup lang="ts">
import { ref, onMounted, shallowRef } from 'vue'
import { VueMonacoEditor, loader } from '@guolao/vue-monaco-editor'
import * as monaco from 'monaco-editor'

// Configure loader to use local monaco-editor
loader.config({ monaco })

const activeCmd = ref(1)
const activeNavTab = ref('All')
const activeResultTab = ref('JSON')

const code = ref(`# 1 — Cluster health
GET /_cluster/health

# 2 — Yellow shard query
GET /_cat/shards?v&h=index,shard,prirep,state`)

const response = ref(`{
  "cluster_name": "prod-us-east-1",
  "status": "green",
  "timed_out": false,
  "number_of_nodes": 3
}`)

const editorOptions = {
  minimap: { enabled: false },
  fontSize: 13,
  lineNumbers: 'on',
  roundedSelection: false,
  scrollBeyondLastLine: false,
  automaticLayout: true,
  theme: 'vs-dark',
  fontFamily: 'var(--esp-font-mono)',
  backgroundColor: '#0a0a0b',
  lineHeight: 22,
  padding: { top: 12 }
}

const resultOptions = {
  ...editorOptions,
  readOnly: true,
  lineNumbers: 'off',
  folding: true
}

const commands = [
  { id: 1, method: 'GET', path: '/_cluster/health', note: 'Cluster health check', time: 'just now', tag: '' },
  { id: 2, method: 'GET', path: '/_cat/shards?v&h=…', note: 'Yellow shard query', time: '12 min', tag: 'shards', bookmarked: true },
  { id: 3, method: 'POST', path: '/logs-prod-*/_search', note: 'Error log search', time: '34 min', tag: 'search' },
  { id: 4, method: 'GET', path: '/_nodes/stats', note: 'Node memory stats', time: '1 hr', tag: '' },
  { id: 5, method: 'PUT', path: '/traces-2024.01/_settings', note: 'Adjust replica count', time: '2 hr', tag: 'index' },
]

const selectCmd = (id: number) => {
  activeCmd.value = id
}

const handleMount = (editor: any) => {
  // Can add custom keybindings or settings here
}
</script>

<template>
  <div class="h-full flex overflow-hidden animate-in fade-in duration-300">
    <!-- Command Navigator (LEFT) -->
    <!-- ... same as before ... -->
    <div id="cmd-nav" class="w-65 min-w-65 bg-bg-2 border-r border-border flex flex-col overflow-hidden flex-shrink-0">
      <div class="p-3 px-3.5 border-b border-border flex-shrink-0">
        <div class="text-12px font-600 tracking-0.04em uppercase text-text-3 mb-2.5">Command Navigator</div>
        <div class="flex items-center gap-1.75 bg-bg-3 border border-border rounded-6px px-2.25 h-7.5 transition-all focus-within:border-accent">
          <div class="w-3 h-3 text-text-3 i-lucide-search"></div>
          <input type="text" placeholder="Filter commands…" class="flex-1 border-none bg-transparent text-text font-sans text-12.5px outline-none placeholder:text-text-3" />
        </div>
      </div>

      <div class="flex gap-0.5 px-3.5 pt-2 flex-shrink-0">
        <div v-for="tab in ['All', 'Saved', 'History']" :key="tab"
          class="text-12px font-500 p-1 px-2.5 rounded-5px cursor-pointer text-text-3 transition-all hover:bg-bg-3 hover:text-text-2"
          :class="{ '!bg-accent-glow !text-accent': activeNavTab === tab }"
          @click="activeNavTab = tab"
        >
          <div v-if="tab === 'Saved'" class="w-2.75 h-2.75 mr-0.75 inline-block i-lucide-bookmark"></div>
          <div v-if="tab === 'History'" class="w-2.75 h-2.75 mr-0.75 inline-block i-lucide-history"></div>
          {{ tab }}
        </div>
      </div>

      <div class="flex-1 overflow-y-auto p-2">
        <div class="text-10px font-600 tracking-0.08em uppercase text-text-3 p-2.5 pb-1.25 opacity-80">This session · 8 commands</div>
        <div v-for="cmd in commands" :key="cmd.id"
          class="p-2 px-2.5 rounded-7px cursor-pointer border border-transparent transition-all mb-0.75 hover:bg-bg-3 hover:border-border"
          :class="{ '!bg-bg-4 !border-border-2': activeCmd === cmd.id }"
          @click="selectCmd(cmd.id)"
        >
          <div class="flex items-center mb-1">
            <span class="inline-flex items-center h-4.5 px-1.5 rounded-4px text-10.5px font-700 font-mono mr-1.5 flex-shrink-0"
              :class="{
                'bg-[rgba(34,197,94,0.12)] text-[#4ade80]': cmd.method === 'GET',
                'bg-[rgba(91,108,248,0.15)] text-[#818cf8]': cmd.method === 'POST',
                'bg-[rgba(234,179,8,0.12)] text-[#fbbf24]': cmd.method === 'PUT',
                'bg-[rgba(239,68,68,0.12)] text-[#f87171]': cmd.method === 'DELETE'
              }"
            >{{ cmd.method }}</span>
            <span class="text-12px font-mono text-text font-500 overflow-hidden text-ellipsis whitespace-nowrap">{{ cmd.path }}</span>
          </div>
          <div class="flex items-center gap-1.5 mt-1.25">
            <span v-if="cmd.tag" class="text-10px px-1.5 rounded-full border border-border text-text-3 bg-bg-3 flex-shrink-0">{{ cmd.tag }}</span>
            <span class="text-11.5px text-text-2 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{{ cmd.note }}</span>
            <span class="text-10.5px text-text-3 font-mono flex-shrink-0">{{ cmd.time }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Editor Area -->
    <div id="editor-area" class="flex-1 flex flex-col overflow-hidden min-w-0 bg-bg">
      <div class="flex items-center gap-2 p-2.5 px-4 border-b border-border bg-bg-2 flex-shrink-0">
        <span class="text-12.5px font-600 text-text-2 flex-1">console.es &nbsp;<span class="text-text-3 font-400 text-11.5px">· 8 commands</span></span>
        <button class="flex items-center gap-1.25 p-1.25 px-3 rounded-6px border border-border bg-transparent text-text-2 font-sans text-12.5px cursor-pointer transition-all hover:bg-bg-3 hover:text-text">
          <div class="w-3.25 h-3.25 i-lucide-align-left"></div>
          Format
        </button>
        <div class="w-px h-4.5 bg-border flex-shrink-0"></div>
        <button class="flex items-center gap-1.25 p-1.25 px-3 rounded-6px border border-accent bg-accent text-white font-sans text-12.5px cursor-pointer transition-all hover:bg-[#6b7cff] hover:border-[#6b7cff]">
          <div class="w-3.25 h-3.25 i-lucide-play"></div>
          Run &nbsp;<span class="opacity-70 text-11px">⌘↵</span>
        </button>
      </div>

      <div class="flex-1 relative overflow-hidden">
        <vue-monaco-editor
          v-model:value="code"
          language="markdown"
          theme="vs-dark"
          :options="editorOptions"
          @mount="handleMount"
        />
      </div>

      <div class="h-6 bg-bg-4 border-t border-border flex items-center px-3.5 gap-4 flex-shrink-0">
        <span class="text-11px font-mono text-green flex items-center gap-1.25">
          <div class="w-2.5 h-2.5 i-lucide-play"></div>
          200 OK · 14ms
        </span>
        <span class="text-11px font-mono text-text-3 ml-auto">prod-us-east-1 · ES 8.12.1</span>
      </div>
    </div>

    <!-- Result Panel (RIGHT) -->
    <div id="result-panel" class="w-95 min-w-95 bg-bg-2 border-l border-border flex flex-col overflow-hidden flex-shrink-0">
      <div class="flex items-center justify-between p-2.5 px-3.5 border-b border-border flex-shrink-0">
        <span class="text-12px font-600 tracking-0.04em uppercase text-text-3">Response</span>
        <span class="text-11.5px font-mono text-green">200 · 14ms</span>
      </div>
      <div class="flex border-b border-border flex-shrink-0">
        <div v-for="tab in ['JSON', 'Table', 'Raw']" :key="tab"
          class="text-12px font-500 p-2 px-3.5 cursor-pointer text-text-3 border-b-2 border-transparent transition-all hover:text-text-2"
          :class="{ '!text-accent !border-accent': activeResultTab === tab }"
          @click="activeResultTab = tab"
        >{{ tab }}</div>
      </div>
      <div class="flex-1 relative overflow-hidden">
        <vue-monaco-editor
          v-model:value="response"
          language="json"
          theme="vs-dark"
          :options="resultOptions"
        />
      </div>
    </div>
  </div>
</template>
