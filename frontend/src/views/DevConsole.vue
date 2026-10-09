<script setup lang="ts">
import { ref, shallowRef, watch, computed } from 'vue'
import { useRoute } from 'vue-router'
import { VueMonacoEditor, loader } from '@guolao/vue-monaco-editor'
import * as monaco from 'monaco-editor'
import { NSplit } from 'naive-ui'
import { useMetadataStore } from '../stores/metadata'

// Configure loader to use local monaco-editor
loader.config({ monaco })

const route = useRoute()
const metadataStore = useMetadataStore()

// 活动集群由 URL 决定（/cluster/:id/console）
const clusterId = computed(() => route.params.id as string)
const activeLine = ref(0)
const navFilter = ref('')
const activeNavTab = ref('All')

// navigator 收起状态 + 编辑/结果分栏比例：落 localStorage 记忆
const NAV_COLLAPSED_KEY = 'espulse:console-nav-collapsed'
const SPLIT_SIZE_KEY = 'espulse:console-split'
const readStored = (key: string) => {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}
const navCollapsed = ref(readStored(NAV_COLLAPSED_KEY) === '1')
const splitSize = ref((() => {
  const n = Number(readStored(SPLIT_SIZE_KEY))
  return n >= 0.3 && n <= 0.8 ? n : 0.65
})())
watch(navCollapsed, (v) => {
  try {
    localStorage.setItem(NAV_COLLAPSED_KEY, v ? '1' : '0')
  } catch {
    // 存不下就退化为仅本次会话有效
  }
})
watch(splitSize, (v) => {
  try {
    localStorage.setItem(SPLIT_SIZE_KEY, String(v))
  } catch {
    // 同上
  }
})
const editorRef = shallowRef<any>(null)
const isLoading = ref(false)
const requestDuration = ref(0)
const requestStatus = ref<number | null>(null)

// Watch for cluster change to fetch metadata
watch(clusterId, (newId) => {
  if (newId) {
    metadataStore.fetchIndices(newId)
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

// Response 只区分 JSON 与纯文本：能解析成 JSON 就按 JSON 高亮，否则按纯文本渲染
const resultLanguage = computed(() => {
  try {
    JSON.parse(response.value)
    return 'json'
  } catch {
    return 'plaintext'
  }
})

// Monaco 要在 canvas 里量字符宽度（font 简写解析不了 var()），所以取一次实际值传进去，
// 否则编辑器会静默用 Monaco 自带默认字体
const MONO_FONT = getComputedStyle(document.documentElement)
  .getPropertyValue('--esp-font-mono')
  .trim() || 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace'

const editorOptions: monaco.editor.IStandaloneEditorConstructionOptions = {
  minimap: { enabled: false },
  fontSize: 13,
  lineNumbers: 'on',
  lineNumbersMinChars: 1,
  glyphMargin: true,
  roundedSelection: false,
  scrollBeyondLastLine: false,
  automaticLayout: true,
  theme: 'vs-dark',
  fontFamily: MONO_FONT,
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
  glyphMargin: false,
  folding: true
}

// 命令目录（TOC）：从编辑器内容实时解析，每项只取命令首行
const CMD_RE = /^(GET|POST|PUT|DELETE|HEAD|PATCH)\s+(.*)$/i
const outline = ref<{ line: number; method: string; text: string }[]>([])

const filteredOutline = computed(() => {
  const q = navFilter.value.trim().toLowerCase()
  if (!q) return outline.value
  return outline.value.filter(
    (c) => c.text.toLowerCase().includes(q) || c.method.toLowerCase().includes(q)
  )
})

const refreshOutline = () => {
  const model = editorRef.value?.getModel()
  if (!model) return
  const items: { line: number; method: string; text: string }[] = []
  for (let i = 1; i <= model.getLineCount(); i++) {
    const m = model.getLineContent(i).trim().match(CMD_RE)
    if (m) items.push({ line: i, method: m[1].toUpperCase(), text: m[2].trim() })
  }
  outline.value = items
}

// 跳转到命令行并短暂高亮落点
let flashDecorations: monaco.editor.IEditorDecorationsCollection | null = null
const jumpTo = (line: number) => {
  const editor = editorRef.value
  if (!editor) return
  editor.revealLineInCenter(line, monaco.editor.ScrollType.Smooth)
  editor.setPosition({ lineNumber: line, column: 1 })
  editor.focus()
  activeLine.value = line
  if (!flashDecorations) flashDecorations = editor.createDecorationsCollection()
  const deco = flashDecorations!
  deco.set([
    {
      range: new monaco.Range(line, 1, line, 1),
      options: { isWholeLine: true, className: 'esp-flash-line' }
    }
  ])
  window.setTimeout(() => deco.clear(), 600)
}

// 光标移动时，目录高亮跟随光标所在命令
const syncActiveFromCursor = (lineNumber: number) => {
  let matched = 0
  for (const c of outline.value) {
    if (c.line <= lineNumber) matched = c.line
    else break
  }
  if (matched) activeLine.value = matched
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
          metadataStore.fetchFields(clusterId.value, indexName)
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
  /*monaco.languages.registerCodeLensProvider(langId, {
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
  })*/
}

const handleMount = (editor: any) => {
  editorRef.value = editor
  registerESLanguage()

  // Glyph Margin：在命令行左侧显示可点击的 ▶ 图标（与 CodeLens 并存）
  const glyphDecorations = editor.createDecorationsCollection()
  const updateGlyphs = () => {
    const model = editor.getModel()
    if (!model) return
    const decorations: monaco.editor.IModelDeltaDecoration[] = []
    for (let i = 1; i <= model.getLineCount(); i++) {
      const content = model.getLineContent(i).trim()
      if (content.match(/^(GET|POST|PUT|DELETE|HEAD|PATCH)\s+/i)) {
        decorations.push({
          range: new monaco.Range(i, 1, i, 1),
          options: {
            glyphMarginClassName: 'esp-run-glyph',
            glyphMarginHoverMessage: { value: 'Run command' }
          }
        })
      }
    }
    glyphDecorations.set(decorations)
  }
  updateGlyphs()
  refreshOutline()
  editor.onDidChangeModelContent(() => {
    updateGlyphs()
    refreshOutline()
  })

  // 点击 Glyph Margin 图标执行该命令行
  editor.onMouseDown((e: monaco.editor.IEditorMouseEvent) => {
    if (e.target.type !== monaco.editor.MouseTargetType.GUTTER_GLYPH_MARGIN || !e.target.position) return
    const line = e.target.position.lineNumber
    const content = editor.getModel()?.getLineContent(line).trim() || ''
    if (content.match(/^(GET|POST|PUT|DELETE|HEAD|PATCH)\s+/i)) {
      editor.setPosition({ lineNumber: line, column: 1 })
      runCommand()
    }
  })

  // 光标移动时同步目录高亮
  editor.onDidChangeCursorPosition((e: monaco.editor.ICursorPositionChangedEvent) => {
    syncActiveFromCursor(e.position.lineNumber)
  })

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
  if (!editorRef.value || !clusterId.value) return
  
  const cmd = parseCurrentCommand(editorRef.value)
  if (!cmd) return

  isLoading.value = true
  const startTime = Date.now()

  try {
    const url = `/api/proxy/${cmd.path.startsWith('/') ? cmd.path.substring(1) : cmd.path}`
    const responseData = await fetch(url, {
      method: cmd.method,
      headers: {
        'X-Cluster-ID': clusterId.value,
        'Content-Type': 'application/json'
      },
      body: cmd.method !== 'GET' && cmd.method !== 'HEAD' && cmd.body ? cmd.body : undefined
    })

    requestDuration.value = Date.now() - startTime
    requestStatus.value = responseData.status

    // ES 的 _cat 等 API 返回纯文本（text/plain），普通 API 返回 JSON。
    // 依据响应头 Content-Type 决定是否解析，避免对纯文本调用 json() 报错。
    const contentType = responseData.headers.get('content-type') || ''
    const raw = await responseData.text()
    if (contentType.includes('application/json')) {
      try {
        response.value = JSON.stringify(JSON.parse(raw), null, 2)
      } catch {
        response.value = raw
      }
    } else {
      response.value = raw
    }
  } catch (err) {
    response.value = JSON.stringify({ error: err instanceof Error ? err.message : String(err) }, null, 2)
    requestStatus.value = 500
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
    <!-- Command Navigator (LEFT)：可收起为图标栏 -->
    <div id="cmd-nav"
      class="bg-bg-2 border-r border-border flex flex-col overflow-hidden flex-shrink-0 transition-all duration-200"
      :class="navCollapsed ? 'w-11' : 'w-65'"
    >
      <!-- 收起态：只留一个展开图标 -->
      <template v-if="navCollapsed">
        <button class="btn-icon mx-auto mt-3 flex-shrink-0" title="展开命令目录" @click="navCollapsed = false">
          <div class="w-3.75 h-3.75 i-lucide-panel-left-open"></div>
        </button>
      </template>

      <!-- 展开态 -->
      <template v-else>
        <div class="p-3 px-3.5 border-b border-border flex-shrink-0">
          <div class="flex items-center justify-between mb-2.5">
            <div class="text-12px font-600 tracking-0.04em uppercase text-text-3">Command Navigator</div>
            <button class="btn-icon -mr-1.5" title="收起命令目录" @click="navCollapsed = true">
              <div class="w-3.75 h-3.75 i-lucide-panel-left-close"></div>
            </button>
          </div>
          <div class="flex items-center gap-1.75 bg-bg-3 border border-border rounded-6px px-2.25 h-7.5 transition-all focus-within:border-accent">
            <div class="w-3 h-3 text-text-3 i-lucide-search"></div>
            <input v-model="navFilter" type="text" placeholder="Filter commands…" class="flex-1 border-none bg-transparent text-text font-sans text-12.5px outline-none placeholder:text-text-3" />
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
          <div class="text-10px font-600 tracking-0.08em uppercase text-text-3 p-2.5 pb-1.25 opacity-80">This session · {{ outline.length }} commands</div>
          <div v-for="cmd in filteredOutline" :key="cmd.line"
            class="flex items-center gap-1.5 p-1.5 px-2.5 rounded-7px cursor-pointer border border-transparent transition-all mb-0.75 hover:bg-bg-3 hover:border-border"
            :class="{ '!bg-bg-4 !border-border-2': activeLine === cmd.line }"
            @click="jumpTo(cmd.line)"
          >
            <span class="inline-flex items-center h-4.5 px-1.5 rounded-4px text-10.5px font-700 font-mono flex-shrink-0"
              :class="{
                'bg-[rgba(34,197,94,0.12)] text-[#4ade80]': cmd.method === 'GET',
                'bg-[rgba(91,108,248,0.15)] text-[#818cf8]': cmd.method === 'POST',
                'bg-[rgba(234,179,8,0.12)] text-[#fbbf24]': cmd.method === 'PUT',
                'bg-[rgba(239,68,68,0.12)] text-[#f87171]': cmd.method === 'DELETE'
              }"
            >{{ cmd.method }}</span>
            <span class="flex-1 text-12px font-mono text-text font-500 overflow-hidden text-ellipsis whitespace-nowrap">{{ cmd.text }}</span>
          </div>
          <div v-if="!filteredOutline.length" class="p-2.5 text-11.5px text-text-3">
            {{ navFilter ? 'No matching commands' : 'No commands' }}
          </div>
        </div>
      </template>
    </div>

    <!-- Editor + Response：可拖动分栏 -->
    <n-split
      v-model:size="splitSize"
      direction="horizontal"
      :min="0.3"
      :max="0.8"
      :resize-trigger-size="6"
      class="flex-1 min-w-0"
      :theme-overrides="{ resizableTriggerColor: 'var(--esp-border)', resizableTriggerColorHover: 'var(--esp-accent)' }"
    >
      <template #1>
        <!-- Editor Area -->
        <div id="editor-area" class="h-full flex flex-col overflow-hidden bg-bg">
          <div class="flex items-center gap-2 p-2.5 px-4 border-b border-border bg-bg-2 flex-shrink-0">
            <span class="text-12.5px font-600 text-text-2 flex-1">console.es &nbsp;<span class="text-text-3 font-400 text-11.5px">· {{ outline.length }} commands</span></span>
            <button
              class="flex items-center gap-1.25 p-1.25 px-3 rounded-6px border border-border bg-transparent text-text-2 font-sans text-12.5px cursor-pointer transition-all hover:bg-bg-3 hover:text-text"
              @click="formatCode"
            >
              <div class="w-3.25 h-3.25 i-lucide-align-left"></div>
              Format
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
        </div>
      </template>

      <template #2>
        <!-- Result Panel (RIGHT)：只区分 JSON / 纯文本 -->
        <div id="result-panel" class="h-full bg-bg-2 flex flex-col overflow-hidden">
          <div class="flex items-center justify-between p-2.5 px-3.5 border-b border-border flex-shrink-0">
            <span class="text-12px font-600 tracking-0.04em uppercase text-text-3">Response</span>
            <span v-if="isLoading" class="text-11.5px font-mono text-text-3 flex items-center gap-1.25">
              <div class="w-3 h-3 i-lucide-loader animate-spin"></div>
              Running…
            </span>
            <span v-else-if="requestStatus" class="text-11.5px font-mono" :class="requestStatus < 400 ? 'text-green' : 'text-red'">
              {{ requestStatus }} · {{ requestDuration }}ms
            </span>
          </div>
          <div class="flex-1 relative overflow-hidden">
            <vue-monaco-editor
              v-model:value="response"
              :language="resultLanguage"
              theme="vs-dark"
              :options="resultOptions"
            />
          </div>
        </div>
      </template>
    </n-split>
  </div>
</template>
