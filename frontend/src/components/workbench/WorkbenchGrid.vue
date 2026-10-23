<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EsIndex, EsShard } from '../../api/catalog'
import { fmtBytes, fmtInt, fmtRate, usageClass } from '../../utils/format'
import type { SortKey, SortOrder, ViewMode, WbGroup, WbIndexCol, WbIndexRow, WbNodeCol, WbNodeRow } from './types'

const { t } = useI18n()

const props = defineProps<{
  view: ViewMode
  nodeColumns: WbNodeCol[]
  indexRows: WbIndexRow[]
  grouped: boolean
  groups: WbGroup[]
  expanded: string[]
  sortKey: SortKey
  sortOrder: SortOrder
  // 节点视角
  indexColumns: WbIndexCol[]
  nodeRows: WbNodeRow[]
  nodePage: number
  nodePages: number
  nodeTotal: number
  nodePageSize: number
}>()

const emit = defineEmits<{
  (e: 'sort', key: SortKey): void
  (e: 'toggle-group-row', prefix: string): void
  (e: 'node-page', delta: number): void
}>()

const isExpanded = (prefix: string) => props.expanded.includes(prefix)

// 单行容量的分片方块数：16px 方块 + 2px 间隔，列宽再扣掉左右各 4px 内边距。
// 索引视角节点列 86px → 4；未分配列 72px → 3；节点视角索引列 64px → 3。
const NODE_CELL_CAP = 4
const UNASSIGNED_CAP = 3
const AXIS_CELL_CAP = 3

// 超出单行容量的分片聚合成「+N」：既不让方块折行把行顶高，也不像原来那样静默丢信息
type ShardView = { shown: EsShard[]; more: number; total: number }
function splitShards(list: EsShard[], cap: number): ShardView {
  if (list.length <= cap) return { shown: list, more: 0, total: list.length }
  return { shown: list.slice(0, cap - 1), more: list.length - cap + 1, total: list.length }
}

// 索引视角的展示行：平铺时逐行索引；聚合时「组头 + （展开后的）子行」。
// 统一成一条 v-for，避免组内子行与平铺行两份重复模板。
type GridRow =
  | { kind: 'group'; group: WbGroup }
  | { kind: 'index'; row: WbIndexRow; child: boolean; cells: ShardView[]; unassigned: ShardView }

const displayRows = computed<GridRow[]>(() => {
  const build = (row: WbIndexRow, child: boolean): GridRow => ({
    kind: 'index',
    row,
    child,
    cells: row.cells.map(cell => splitShards(cell ?? [], NODE_CELL_CAP)),
    unassigned: splitShards(row.unassigned, UNASSIGNED_CAP),
  })
  if (!props.grouped) return props.indexRows.map(row => build(row, false))
  const out: GridRow[] = []
  for (const group of props.groups) {
    out.push({ kind: 'group', group })
    if (props.expanded.includes(group.prefix)) {
      for (const row of group.rows) out.push(build(row, true))
    }
  }
  return out
})

// 节点视角：行 = 节点、列 = 索引，列更窄，单行只放得下 3 个方块
const nodeCells = computed(() =>
  props.nodeRows.map(row => row.cells.map(cell => splitShards(cell ?? [], AXIS_CELL_CAP))),
)

function sortArrow(key: SortKey): string {
  if (props.sortKey !== key) return ''
  return props.sortOrder === 'asc' ? ' ↑' : ' ↓'
}

const shardState = (s: EsShard) => {
  const st = (s.state || '').toLowerCase()
  if (st === 'relocating') return 'relo'
  if (st === 'initializing') return 'init'
  if (st === 'unassigned') return 'un'
  return 'started'
}

function shardTip(s: EsShard): string {
  const parts = [s.index, `#${s.shard}`, s.prirep === 'p' ? t('vitals.pri') : t('vitals.rep'), s.state]
  if (s.node) parts.push('@ ' + s.node)
  if (s['unassigned.reason']) parts.push(`(${s['unassigned.reason']})`)
  return parts.join(' ')
}

function unassignedTip(s: EsShard): string {
  const parts = [s.index, `#${s.shard}`, s.prirep === 'p' ? t('vitals.pri') : t('vitals.rep')]
  if (s['unassigned.reason']) parts.push(`(${s['unassigned.reason']})`)
  return parts.join(' ')
}

const healthClass = (h: string) => {
  const k = (h || '').toLowerCase()
  return k === 'green' || k === 'yellow' || k === 'red' ? k : 'green'
}

// 索引单元格只有一行，元信息必须极短：文档数以 k/M/B 简写、存储去掉空格与尾部单位字母，
// 完整值放 title，避免长数字撑破 326px 的索引列。
function fmtCount(value: string | number | undefined | null): string {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  const unit = n >= 1e9 ? 'B' : n >= 1e6 ? 'M' : n >= 1e3 ? 'k' : ''
  if (!unit) return String(n)
  const scaled = n / (unit === 'B' ? 1e9 : unit === 'M' ? 1e6 : 1e3)
  return `${scaled >= 100 ? Math.round(scaled) : scaled.toFixed(1)}${unit}`
}

function fmtStore(value: string | number | undefined | null): string {
  const s = fmtBytes(value)
  const [num, unit] = s.split(' ')
  if (!unit) return s
  const short = unit === 'B' ? unit : unit.slice(0, -1)
  return `${num.endsWith('.0') ? num.slice(0, -2) : num}${short}`
}

// 非 open 索引（close / frozen）才需要显性提示；缺值按 open 处理，不占位
const isClosed = (status: string | undefined) => (status || 'open').toLowerCase() !== 'open'

const priRepTip = (pri: string | number, rep: string | number) =>
  `${t('vitals.pri')} ${pri} / ${t('vitals.rep')} ${rep}`

// 副本 0 = 冗余缺口：health 仍可能是 green，只有主分片所在节点挂掉才会暴露，所以要主动提醒。
// close / frozen 索引的副本数不代表在线冗余，不打扰。
const noReplica = (index: EsIndex) => !isClosed(index.status) && Number(index.rep) === 0
const priRepTipFor = (index: EsIndex) =>
  noReplica(index) ? `${priRepTip(index.pri, index.rep)} · ${t('grid.noReplica')}` : priRepTip(index.pri, index.rep)
const groupPriRepTip = (g: WbGroup) =>
  g.repZero ? `${priRepTip(g.pri, g.rep)} · ${t('grid.noReplicaCount', { n: g.repZero })}` : priRepTip(g.pri, g.rep)
const docsTip = (value: string | number | undefined | null) => `${t('grid.docs')} ${fmtInt(value)}`
const storeTip = (value: string | number | undefined | null) => `${t('grid.store')} ${fmtBytes(value)}`
const moreTip = (n: number) => t('grid.moreShards', { n })

// 用量轨宽度：缺值按 0 处理，越界值夹到 0–100，避免脏数据把轨道撑破
function pct(value: string | number | undefined): string {
  const n = Number(value)
  if (!Number.isFinite(n)) return '0%'
  return `${Math.min(100, Math.max(0, n))}%`
}
</script>

<template>
  <!-- ============ 索引视角：行=索引，列=节点 ============ -->
  <template v-if="view === 'index'">
    <table class="wgrid">
      <colgroup>
        <col style="width: 300px" />
        <col style="width: 56px" />
        <col style="width: 68px" />
        <col style="width: 72px" />
        <col style="width: 64px" />
        <col style="width: 64px" />
        <col v-for="n in nodeColumns" :key="n.name" style="width: 86px" />
        <col style="width: 72px" />
      </colgroup>
      <thead>
        <tr class="h1">
          <th class="frz" colspan="6" style="text-align: left">
            {{ t('grid.indexSummary') }} ({{ indexRows.length }})
          </th>
          <th v-if="nodeColumns.length" class="grp" :colspan="nodeColumns.length">{{ t('grid.nodeShardDist') }}</th>
          <th class="grp">{{ t('grid.unassigned') }}</th>
        </tr>
        <tr>
          <th class="frz sortable" :class="{ sorted: sortKey === 'index' }" @click="emit('sort', 'index')">
            {{ t('grid.index') }}{{ sortArrow('index') }}
          </th>
          <th class="num">{{ t('grid.priRep') }}</th>
          <th class="num sortable" :class="{ sorted: sortKey === 'docs' }" @click="emit('sort', 'docs')">
            {{ t('grid.docs') }}{{ sortArrow('docs') }}
          </th>
          <th class="num sortable" :class="{ sorted: sortKey === 'store' }" @click="emit('sort', 'store')">
            {{ t('grid.store') }}{{ sortArrow('store') }}
          </th>
          <th class="num sortable" :class="{ sorted: sortKey === 'write' }" @click="emit('sort', 'write')">
            {{ t('grid.writePerSec') }}{{ sortArrow('write') }}
          </th>
          <th class="num sortable" :class="{ sorted: sortKey === 'read' }" @click="emit('sort', 'read')">
            {{ t('grid.readPerSec') }}{{ sortArrow('read') }}
          </th>
          <th v-for="n in nodeColumns" :key="n.name" class="node">
            <div class="nn">
              <span v-if="n.master" class="star">★</span>{{ n.name }}
            </div>
            <div class="st">
              <span :class="usageClass(n.heap, 85, 70)">H<b>{{ n.heap || '-' }}</b></span>
              <span :class="usageClass(n.cpu, 80, 65)">C<b>{{ n.cpu || '-' }}</b></span>
              <span :class="usageClass(n.disk, 80, 70)">D<b>{{ n.disk || '-' }}</b></span>
            </div>
          </th>
          <th class="node">
            <div class="nn">{{ t('grid.unassigned') }}</div>
            <div class="st">pseudo</div>
          </th>
        </tr>
      </thead>
      <tbody>
        <template v-for="item in displayRows" :key="item.kind === 'group' ? 'g:' + item.group.prefix : 'i:' + item.row.index.index">
          <!-- 组头行（按前缀聚合） -->
          <tr v-if="item.kind === 'group'" class="group" @click="emit('toggle-group-row', item.group.prefix)">
            <td class="frz index">
              <div class="ixcell">
                <span class="caret" :class="{ open: isExpanded(item.group.prefix) }">
                  <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="9 18 15 12 9 6" />
                  </svg>
                </span>
                <span class="hdot" :class="item.group.worst"></span>
                <span class="nm">{{ item.group.prefix }}*</span>
                <!-- 组头行宽 = 组内索引数；其余指标走独立的表格列 -->
                <span class="gcnt">{{ item.group.rows.length }}×</span>
              </div>
            </td>
            <td
              class="num mono gval"
              :class="{ 'no-rep': item.group.repZero > 0 }"
              :title="groupPriRepTip(item.group)"
            >{{ item.group.pri }}/{{ item.group.rep }}</td>
            <td class="num mono gval" :title="docsTip(item.group.docs)">{{ fmtCount(item.group.docs) }}</td>
            <td class="num mono gval" :title="storeTip(item.group.store)">{{ fmtStore(item.group.store) }}</td>
            <td class="num gval">Σ</td>
            <td class="num gval">Σ</td>
            <td v-for="n in nodeColumns" :key="n.name" class="node dim">·</td>
            <td class="node dim">·</td>
          </tr>

          <!-- 索引行（含聚合展开的子行） -->
          <tr v-else :class="{ child: item.child }">
            <td class="frz index">
              <div class="ixcell">
                <span class="hdot" :class="healthClass(item.row.index.health)"></span>
                <span class="nm" :title="item.row.index.index">{{ item.row.index.index }}</span>
                <!-- 状态常态不占位：只有 close / frozen 等异常态才冒出来 -->
                <span v-if="isClosed(item.row.index.status)" class="ixst">{{ item.row.index.status }}</span>
              </div>
            </td>
            <td
              class="num mono"
              :class="{ 'no-rep': noReplica(item.row.index) }"
              :title="priRepTipFor(item.row.index)"
            >{{ item.row.index.pri }}/<span class="repdigit">{{ item.row.index.rep }}</span></td>
            <td class="num mono" :title="docsTip(item.row.index['docs.count'])">{{ fmtCount(item.row.index['docs.count']) }}</td>
            <td class="num mono" :title="storeTip(item.row.index['store.size'])">{{ fmtStore(item.row.index['store.size']) }}</td>
            <td class="num mono">{{ fmtRate(item.row.index.index_rate) }}</td>
            <td class="num mono">{{ fmtRate(item.row.index.search_rate) }}</td>
            <td v-for="(cell, ci) in item.cells" :key="ci" class="node">
              <div v-if="cell.total" class="shardcell">
                <div v-for="(s, si) in cell.shown" :key="si" class="shard" :class="[shardState(s), s.prirep === 'p' ? '' : 'rep']" :title="shardTip(s)">
                  <span>{{ s.shard }}</span>
                </div>
                <div v-if="cell.more" class="shard more" :title="moreTip(cell.more)">+{{ cell.more }}</div>
              </div>
              <span v-else class="dim micro">·</span>
            </td>
            <td class="node">
              <div v-if="item.unassigned.total" class="shardcell">
                <span v-for="(s, si) in item.unassigned.shown" :key="si" class="shard un" :title="unassignedTip(s)">{{ s.shard }}</span>
                <span v-if="item.unassigned.more" class="shard more" :title="moreTip(item.unassigned.more)">+{{ item.unassigned.more }}</span>
              </div>
              <span v-else class="dim micro">·</span>
            </td>
          </tr>
        </template>
      </tbody>
    </table>
  </template>

  <!-- ============ 节点视角：行=节点，列=索引（分页） ============ -->
  <template v-else>
    <div class="axisbar">
      <span>{{ t('grid.indexAxisLabel') }}</span>
      <span class="sp"></span>
      <button :disabled="nodePage <= 1" @click="emit('node-page', -1)">{{ t('grid.prevPage') }}</button>
      <span>{{ t('grid.pageInfo', { page: nodePage, pages: nodePages, total: nodeTotal }) }}</span>
      <button :disabled="nodePage >= nodePages" @click="emit('node-page', 1)">+{{ nodePageSize }}</button>
    </div>
    <table class="wgrid is-node">
      <colgroup>
        <col style="width: 150px" />
        <col style="width: 96px" />
        <col style="width: 52px" />
        <col style="width: 52px" />
        <!-- 156px：11px 等宽字宽 6.6px，容得下 "99.99 / 99.99 / 99.99"（21 字 = 138.6px）+ 左右各 8px 内边距 -->
        <col style="width: 156px" />
        <col style="width: 56px" />
        <col style="width: 62px" />
        <col v-for="c in indexColumns" :key="c.name" style="width: 64px" />
      </colgroup>
      <thead>
        <tr class="h1">
          <th class="frz" colspan="7" style="text-align: left">{{ t('grid.nodeSummary') }} ({{ nodeRows.length }})</th>
          <th v-if="indexColumns.length" class="grp" :colspan="indexColumns.length">
            {{ t('grid.indexAxis', { page: nodePage, pages: nodePages, size: nodePageSize }) }}
          </th>
        </tr>
        <tr>
          <th class="frz">{{ t('grid.nodeRole') }}</th>
          <th class="num">{{ t('grid.heap') }}</th>
          <th class="num">{{ t('grid.cpu') }}</th>
          <th class="num">{{ t('grid.disk') }}</th>
          <th>{{ t('grid.load') }}</th>
          <th class="num">{{ t('grid.shards') }}</th>
          <th class="num">{{ t('grid.unassigned') }}</th>
          <th v-for="c in indexColumns" :key="c.name" class="node" :title="c.name">
            <div class="nn">{{ c.name.length > 9 ? c.name.slice(0, 8) + '…' : c.name }}</div>
            <div class="st">{{ c.health }}</div>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(n, ni) in nodeRows" :key="n.name">
          <td class="frz">
            <div class="nodename">
              <span class="hdot" :class="n.master ? 'yellow' : 'green'"></span> {{ n.name }}<span
                v-if="n.master"
                class="star"
                :title="t('grid.master')"
              >★</span>
            </div>
            <div class="dim micro">{{ n.addr }}</div>
          </td>
          <td class="num usage">
            <div class="umeter" :class="usageClass(n.heap, 85, 70)" :title="t('grid.heap') + ' ' + (n.heap ? n.heap + '%' : '-') + (n.heapMax ? ' / ' + n.heapMax : '')">
              <span class="uv">
                <span v-if="n.heapMax" class="umax">{{ n.heapMax }}</span>{{ n.heap ? n.heap + '%' : '-' }}
              </span>
              <i class="ubar"><b :style="{ width: pct(n.heap) }"></b></i>
            </div>
          </td>
          <td class="num usage">
            <div class="umeter" :class="usageClass(n.cpu, 80, 65)" :title="t('grid.cpu') + ' ' + (n.cpu ? n.cpu + '%' : '-')">
              <span class="uv">{{ n.cpu ? n.cpu + '%' : '-' }}</span>
              <i class="ubar"><b :style="{ width: pct(n.cpu) }"></b></i>
            </div>
          </td>
          <td class="num usage">
            <div class="umeter" :class="usageClass(n.disk, 80, 70)" :title="t('grid.disk') + ' ' + (n.disk ? n.disk + '%' : '-')">
              <span class="uv">{{ n.disk ? n.disk + '%' : '-' }}</span>
              <i class="ubar"><b :style="{ width: pct(n.disk) }"></b></i>
            </div>
          </td>
          <td class="mono">{{ n.load1 || '-' }} / {{ n.load5 || '-' }} / {{ n.load15 || '-' }}</td>
          <td class="num mono">{{ n.shards ?? '-' }}</td>
          <td class="num mono dim">—</td>
          <td v-for="(cell, ci) in nodeCells[ni]" :key="ci" class="node">
            <div v-if="cell.total" class="shardcell">
              <span v-for="(s, si) in cell.shown" :key="si" class="shard" :class="[shardState(s), s.prirep === 'p' ? '' : 'rep']" :title="shardTip(s)">{{ s.shard }}</span>
              <span v-if="cell.more" class="shard more" :title="moreTip(cell.more)">+{{ cell.more }}</span>
            </div>
            <span v-else class="dim micro">·</span>
          </td>
        </tr>
      </tbody>
    </table>
  </template>
</template>
