<script setup lang="ts">
import { ref, onMounted, shallowRef, onBeforeUnmount, watch } from 'vue'
import { VueMonacoEditor, loader } from '@guolao/vue-monaco-editor'
import * as monaco from 'monaco-editor'
import { useClusterStore } from '../stores/cluster'
import { useMetadataStore } from '../stores/metadata'

// Configure loader to use local monaco-editor
loader.config({ monaco })

const clusterStore = useClusterStore()
const metadataStore = useMetadataStore()
const activeCmd = ref(1)
const activeNavTab = ref('All')
const activeResultTab = ref('JSON')
const editorRef = shallowRef<any>(null)
const isLoading = ref(false)
const requestDuration = ref(0)
const requestStatus = ref<number | null>(null)
const requestStatusText = ref('')

// Watch for cluster change to fetch metadata
watch(() => clusterStore.currentClusterId, (newId) => {
  if (newId) {
    metadataStore.fetchIndices()
  }
}, { immediate: true })

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

const editorOptions: monaco.editor.IStandaloneEditorConstructionOptions = {
  minimap: { enabled: false },
  fontSize: 13,
  lineNumbers: 'on',
  roundedSelection: false,
  scrollBeyondLastLine: false,
  automaticLayout: true,
  theme: 'vs-dark',
  fontFamily: 'var(--esp-font-mono)',
  lineHeight: 22,
  padding: { top: 12 },
  wordWrap: 'on',
  formatOnPaste: true,
  suggestSelection: 'first',
  codeLens: true,
  'semanticHighlighting.enabled': true
}

const resultOptions: monaco.editor.IStandaloneEditorConstructionOptions = {
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

// Register custom ES Console language
const registerESLanguage = () => {
  const langId = 'es-console'
  
  // Check if language is already registered
  if (monaco.languages.getLanguages().some(lang => lang.id === langId)) {
    return
  }

  monaco.languages.register({ id: langId })

  // 1. Syntax Highlighting (Monarch)
  monaco.languages.setMonarchTokensProvider(langId, {
    tokenizer: {
      root: [
        [/^\s*(GET|POST|PUT|DELETE|HEAD|PATCH)\b/, 'keyword'],
        [/^\s*#.*$/, 'comment'],
        [/^(\/.*)$/, 'type.identifier'],
        [/[{}]/, 'delimiter.bracket'],
        [/[\[\]]/, 'delimiter.square'],
        [/"[^"]*"/, 'string'],
        [/\b\d+\b/, 'number'],
        [/\b(true|false|null)\b/, 'keyword'],
        [/:/, 'operator'],
        [/,/, 'delimiter']
      ]
    }
  })

  // 2. Completion Provider
  monaco.languages.registerCompletionItemProvider(langId, {
    triggerCharacters: ['/', ' ', '"', '_'],
    provideCompletionItems: (model, position) => {
      const lineContent = model.getLineContent(position.lineNumber)
      const word = model.getWordUntilPosition(position)
      const range = {
        startLineNumber: position.lineNumber,
        endLineNumber: position.lineNumber,
        startColumn: word.startColumn,
        endColumn: word.endColumn
      }

      // Basic HTTP Methods at start of line
      if (position.column <= 10 && !lineContent.trim().startsWith('/')) {
        return {
          suggestions: [
            { label: 'GET', kind: monaco.languages.CompletionItemKind.Keyword, insertText: 'GET ', range },
            { label: 'POST', kind: monaco.languages.CompletionItemKind.Keyword, insertText: 'POST ', range },
            { label: 'PUT', kind: monaco.languages.CompletionItemKind.Keyword, insertText: 'PUT ', range },
            { label: 'DELETE', kind: monaco.languages.CompletionItemKind.Keyword, insertText: 'DELETE ', range }
          ]
        }
      }

      // Basic ES API Endpoints
      const suggestions: monaco.languages.CompletionItem[] = [
        { label: '_cat/indices', kind: monaco.languages.CompletionItemKind.Method, insertText: '_cat/indices?v', range, detail: 'List all indices' },
        { label: '_cat/nodes', kind: monaco.languages.CompletionItemKind.Method, insertText: '_cat/nodes?v', range, detail: 'List all nodes' },
        { label: '_cat/health', kind: monaco.languages.CompletionItemKind.Method, insertText: '_cat/health?v', range, detail: 'Cluster health' },
        { label: '_cluster/health', kind: monaco.languages.CompletionItemKind.Method, insertText: '_cluster/health', range, detail: 'Detailed cluster health' },
        { label: '_search', kind: monaco.languages.CompletionItemKind.Method, insertText: '_search', range, detail: 'Search API' },
        { label: '_mapping', kind: monaco.languages.CompletionItemKind.Method, insertText: '_mapping', range, detail: 'Get index mapping' },
        { label: '_settings', kind: monaco.languages.CompletionItemKind.Method, insertText: '_settings', range, detail: 'Get index settings' }
      ]

      // Index suggestions
      if (lineContent.includes('/')) {
        metadataStore.indices.forEach(idx => {
          suggestions.push({
            label: idx,
            kind: monaco.languages.CompletionItemKind.Folder,
            insertText: idx,
            range,
            detail: 'Index'
          })
        })
      }

      // Field suggestions within JSON body
      if (lineContent.trim().startsWith('"') || lineContent.trim().startsWith('{')) {
        // Try to find the nearest index in previous lines to fetch relevant fields
        let indexName = ''
        for (let i = position.lineNumber; i >= 1; i--) {
          const content = model.getLineContent(i).trim()
          const match = content.match(/^(GET|POST|PUT|DELETE|HEAD|PATCH)\s+([^\/\s]+)/i)
          if (match && !match[2].startsWith('_')) {
            indexName = match[2]
            break
          }
        }

        if (indexName) {
          metadataStore.fetchFields(indexName)
          const fields = metadataStore.fields[indexName] || []
          fields.forEach(f => {
            suggestions.push({
              label: f,
              kind: monaco.languages.CompletionItemKind.Field,
              insertText: f,
              range,
              detail: `Field (${indexName})`
            })
          })
        }
      }

      return { suggestions }
    }
  })

  // 3. CodeLens Provider (Run button above command)
  monaco.languages.registerCodeLensProvider(langId, {
    provideCodeLenses: (model) => {
      const lenses: monaco.languages.CodeLens[] = []
      const lines = model.getLineCount()

      for (let i = 1; i <= lines; i++) {
        const content = model.getLineContent(i).trim()
        if (content.match(/^(GET|POST|PUT|DELETE|HEAD|PATCH)\s+/i)) {
          lenses.push({
            range: {
              startLineNumber: i,
              startColumn: 1,
              endLineNumber: i,
              endColumn: 1
            },
            command: {
              id: 'espulse.runCommand',
              title: '▶ Run',
              arguments: [i]
            }
          })
        }
      }
      return { lenses, dispose: () => {} }
    }
  })
}

const handleMount = (editor: any) => {
  editorRef.value = editor
  registerESLanguage()

  // Register global command for CodeLens (if not already registered)
  // Note: monaco.editor.registerCommand is the official way to register commands by ID
  try {
    monaco.editor.registerCommand('espulse.runCommand', (_accessor: any, lineNumber: number) => {
      // Set cursor to the command line and run
      if (editorRef.value) {
        editorRef.value.setPosition({ lineNumber, column: 1 })
        runCommand()
      }
    })
  } catch (e) {
    // Command might already be registered, which is fine
    console.debug('Command espulse.runCommand already registered or failed to register:', e)
  }

  // Add keyboard shortcut for Run Command
  editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () => {
    runCommand()
  })
}

interface ESCommand {
  method: string
  path: string
  body: string
}

const parseCurrentCommand = (editor: any): ESCommand | null => {
  const model = editor.getModel()
  const position = editor.getPosition()
  const currentLine = position.lineNumber

  let methodLine = -1
  let method = ''
  let path = ''

  // Look upwards for the method/path line
  for (let i = currentLine; i >= 1; i--) {
    const content = model.getLineContent(i).trim()
    const match = content.match(/^(GET|POST|PUT|DELETE|HEAD|PATCH)\s+(.*)$/i)
    if (match) {
      methodLine = i
      method = match[1].toUpperCase()
      path = match[2]
      break
    }
    // If we hit another command's body or a comment, maybe stop? 
    // For now, let's keep it simple.
  }

  if (methodLine === -1) return null

  // Look downwards for the JSON body
  let bodyLines = []
  let braceCount = 0
  let foundStart = false

  for (let i = methodLine + 1; i <= model.getLineCount(); i++) {
    const content = model.getLineContent(i).trim()
    if (content.startsWith('#') || content.match(/^(GET|POST|PUT|DELETE|HEAD|PATCH)\s+/i)) {
      break // New command or comment starts
    }
    
    bodyLines.push(model.getLineContent(i))
    
    // Simple brace matching to find end of JSON
    if (content.includes('{')) {
      if (!foundStart) foundStart = true
      braceCount += (content.match(/{/g) || []).length
    }
    if (content.includes('}')) {
      braceCount -= (content.match(/}/g) || []).length
    }
    
    if (foundStart && braceCount === 0) break
  }

  return {
    method,
    path,
    body: bodyLines.join('\n').trim()
  }
}

const runCommand = async () => {
  if (!editorRef.value || !clusterStore.currentClusterId) return
  
  const cmd = parseCurrentCommand(editorRef.value)
  if (!cmd) return

  isLoading.value = true
  const startTime = Date.now()

  try {
    const url = `/api/proxy/${cmd.path.startsWith('/') ? cmd.path.substring(1) : cmd.path}`
    const responseData = await fetch(url, {
      method: cmd.method,
      headers: {
        'X-Cluster-ID': clusterStore.currentClusterId,
        'Content-Type': 'application/json'
      },
      body: cmd.method !== 'GET' && cmd.method !== 'HEAD' && cmd.body ? cmd.body : undefined
    })

    requestDuration.value = Date.now() - startTime
    requestStatus.value = responseData.status
    requestStatusText.value = responseData.statusText

    const data = await responseData.json()
    response.value = JSON.stringify(data, null, 2)
  } catch (err) {
    response.value = JSON.stringify({ error: err instanceof Error ? err.message : String(err) }, null, 2)
    requestStatus.value = 500
    requestStatusText.value = 'Error'
  } finally {
    isLoading.value = false
  }
}

const formatCode = () => {
  if (!editorRef.value) return
  const model = editorRef.value.getModel()
  const lines = model.getLineCount()
  let newContent = []
  
  for (let i = 1; i <= lines; i++) {
    const line = model.getLineContent(i)
    const trimmed = line.trim()
    
    // If it looks like JSON start, try to format until end of block
    if (trimmed.startsWith('{')) {
      let jsonBlock = [line]
      let braceCount = (trimmed.match(/{/g) || []).length - (trimmed.match(/}/g) || []).length
      let j = i + 1
      
      while (j <= lines && braceCount > 0) {
        const nextLine = model.getLineContent(j)
        jsonBlock.push(nextLine)
        braceCount += (nextLine.match(/{/g) || []).length - (nextLine.match(/}/g) || []).length
        j++
      }
      
      try {
        const formatted = JSON.stringify(JSON.parse(jsonBlock.join('\n')), null, 2)
        newContent.push(formatted)
        i = j - 1 // Skip formatted lines
      } catch (e) {
        newContent.push(line)
      }
    } else {
      newContent.push(line)
    }
  }
  
  editorRef.value.setValue(newContent.join('\n'))
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
        <button 
          class="flex items-center gap-1.25 p-1.25 px-3 rounded-6px border border-border bg-transparent text-text-2 font-sans text-12.5px cursor-pointer transition-all hover:bg-bg-3 hover:text-text"
          @click="formatCode"
        >
          <div class="w-3.25 h-3.25 i-lucide-align-left"></div>
          Format
        </button>
        <div class="w-px h-4.5 bg-border flex-shrink-0"></div>
        <button 
          class="flex items-center gap-1.25 p-1.25 px-3 rounded-6px border border-accent bg-accent text-white font-sans text-12.5px cursor-pointer transition-all hover:bg-[#6b7cff] hover:border-[#6b7cff] disabled:opacity-50 disabled:cursor-not-allowed"
          :disabled="isLoading || !clusterStore.currentClusterId"
          @click="runCommand"
        >
          <div v-if="isLoading" class="w-3.25 h-3.25 i-lucide-loader animate-spin"></div>
          <div v-else class="w-3.25 h-3.25 i-lucide-play"></div>
          {{ isLoading ? 'Running...' : 'Run' }} &nbsp;<span class="opacity-70 text-11px">⌘↵</span>
        </button>
      </div>

      <div class="flex-1 relative overflow-hidden">
        <vue-monaco-editor
          v-model:value="code"
          language="es-console"
          theme="vs-dark"
          :options="editorOptions"
          @mount="handleMount"
        />
      </div>

      <div class="h-6 bg-bg-4 border-t border-border flex items-center px-3.5 gap-4 flex-shrink-0">
        <span v-if="requestStatus" class="text-11px font-mono flex items-center gap-1.25"
          :class="requestStatus < 400 ? 'text-green' : 'text-red'">
          <div v-if="isLoading" class="w-2.5 h-2.5 i-lucide-loader animate-spin"></div>
          <div v-else class="w-2.5 h-2.5 i-lucide-play"></div>
          {{ requestStatus }} {{ requestStatusText }} · {{ requestDuration }}ms
        </span>
        <span v-else-if="isLoading" class="text-11px font-mono text-text-3 flex items-center gap-1.25">
          <div class="w-2.5 h-2.5 i-lucide-loader animate-spin"></div>
          Executing...
        </span>
        <span class="text-11px font-mono text-text-3 ml-auto">
          {{ clusterStore.currentCluster?.name || 'No Cluster' }}
        </span>
      </div>
    </div>

    <!-- Result Panel (RIGHT) -->
    <div id="result-panel" class="w-95 min-w-95 bg-bg-2 border-l border-border flex flex-col overflow-hidden flex-shrink-0">
      <div class="flex items-center justify-between p-2.5 px-3.5 border-b border-border flex-shrink-0">
        <span class="text-12px font-600 tracking-0.04em uppercase text-text-3">Response</span>
        <span v-if="requestStatus" class="text-11.5px font-mono" :class="requestStatus < 400 ? 'text-green' : 'text-red'">
          {{ requestStatus }} · {{ requestDuration }}ms
        </span>
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
