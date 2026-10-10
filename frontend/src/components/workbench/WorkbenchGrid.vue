<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EsShard } from '../../api/catalog'
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

// 索引视角的展示行：平铺时逐行索引；聚合时「组头 + （展开后的）子行」。
// 统一成一条 v-for，避免组内子行与平铺行两份重复模板。
type GridRow =
  | { kind: 'group'; group: WbGroup }
  | { kind: 'index'; row: WbIndexRow; child: boolean }

const displayRows = computed<GridRow[]>(() => {
  if (!props.grouped) return props.indexRows.map(row => ({ kind: 'index' as const, row, child: false }))
  const out: GridRow[] = []
  for (const group of props.groups) {
    out.push({ kind: 'group', group })
    if (props.expanded.includes(group.prefix)) {
      for (const row of group.rows) out.push({ kind: 'index', row, child: true })
    }
  }
  return out
})

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
</script>

<template>
  <!-- ============ 索引视角：行=索引，列=节点 ============ -->
  <template v-if="view === 'index'">
    <table class="wgrid">
      <colgroup>
        <col style="width: 326px" />
        <col style="width: 64px" />
        <col style="width: 64px" />
        <col v-for="n in nodeColumns" :key="n.name" style="width: 86px" />
        <col style="width: 72px" />
      </colgroup>
      <thead>
        <tr class="h1">
          <th class="frz" colspan="3" style="text-align: left">
            {{ t('grid.indexSummary') }} ({{ indexRows.length }})
          </th>
          <th v-if="nodeColumns.length" class="grp" :colspan="nodeColumns.length">{{ t('grid.nodeShardDist') }}</th>
          <th class="grp">{{ t('grid.unassigned') }}</th>
        </tr>
        <tr>
          <th class="frz sortable" :class="{ sorted: sortKey === 'index' }" @click="emit('sort', 'index')">
            {{ t('grid.index') }}{{ sortArrow('index') }}
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
                <div class="ixline1">
                  <span class="caret" :class="{ open: isExpanded(item.group.prefix) }">
                    <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                      <polyline points="9 18 15 12 9 6" />
                    </svg>
                  </span>
                  <span class="hdot" :class="item.group.worst"></span>
                  <span class="nm">{{ item.group.prefix }}*</span>
                </div>
                <!-- 第二行（状态 / 主副 / 文档 / 存储）暂时注释，评估单行密度 -->
                <!-- <div class="ixline2">
                  <span class="mono">{{ item.group.rows.length }}×</span>
                  <span class="dotsep">·</span>
                  <span>{{ t('vitals.pri') }}<b>{{ item.group.pri }}</b>/{{ t('vitals.rep') }}<b>{{ item.group.rep }}</b></span>
                  <span class="dotsep">·</span>
                  <span>{{ t('grid.docs') }}<b>{{ fmtInt(item.group.docs) }}</b></span>
                  <span class="dotsep">·</span>
                  <span>{{ t('grid.store') }}<b>{{ fmtBytes(item.group.store) }}</b></span>
                </div> -->
              </div>
            </td>
            <td class="num dim">Σ</td>
            <td class="num dim">Σ</td>
            <td v-for="n in nodeColumns" :key="n.name" class="node dim">·</td>
            <td class="node dim">·</td>
          </tr>

          <!-- 索引行（含聚合展开的子行） -->
          <tr v-else :class="{ child: item.child }">
            <td class="frz index">
              <div class="ixcell">
                <div class="ixline1">
                  <span class="hdot" :class="healthClass(item.row.index.health)"></span>
                  <span class="nm" :title="item.row.index.index">{{ item.row.index.index }}</span>
                </div>
                <!-- 第二行（状态 / 主副 / 文档 / 存储）暂时注释，评估单行密度 -->
                <!-- <div class="ixline2">
                  <span class="mono" :class="`st-${item.row.index.status}`">{{ item.row.index.status }}</span>
                  <span class="dotsep">·</span>
                  <span>{{ t('vitals.pri') }}<b>{{ item.row.index.pri }}</b>/{{ t('vitals.rep') }}<b>{{ item.row.index.rep }}</b></span>
                  <span class="dotsep">·</span>
                  <span>{{ t('grid.docs') }}<b>{{ fmtInt(item.row.index['docs.count']) }}</b></span>
                  <span class="dotsep">·</span>
                  <span>{{ t('grid.store') }}<b>{{ fmtBytes(item.row.index['store.size']) }}</b></span>
                </div> -->
              </div>
            </td>
            <td class="num mono">{{ fmtRate(item.row.index.index_rate) }}</td>
            <td class="num mono">{{ fmtRate(item.row.index.search_rate) }}</td>
            <td v-for="(cell, ci) in item.row.cells" :key="ci" class="node">
              <div v-if="cell && cell.length" class="shardcell">
                <div v-for="(s, si) in cell.slice(0, 8)" :key="si" class="shard" :class="[shardState(s), s.prirep === 'p' ? '' : 'rep']" :title="shardTip(s)">
                  <span>{{ s.shard }}</span>
                </div>
              </div>
              <span v-else class="dim" style="font-size: 9px">·</span>
            </td>
            <td class="node">
              <div v-if="item.row.unassigned.length" class="shardcell">
                <span v-for="(s, si) in item.row.unassigned.slice(0, 8)" :key="si" class="shard un" :title="unassignedTip(s)">{{ s.shard }}</span>
              </div>
              <span v-else class="dim" style="font-size: 9px">·</span>
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
        <col style="width: 60px" />
        <col style="width: 52px" />
        <col style="width: 52px" />
        <col style="width: 52px" />
        <col style="width: 120px" />
        <col style="width: 56px" />
        <col style="width: 62px" />
        <col v-for="c in indexColumns" :key="c.name" style="width: 64px" />
      </colgroup>
      <thead>
        <tr class="h1">
          <th class="frz" colspan="8" style="text-align: left">{{ t('grid.nodeSummary') }} ({{ nodeRows.length }})</th>
          <th v-if="indexColumns.length" class="grp" :colspan="indexColumns.length">
            {{ t('grid.indexAxis', { page: nodePage, pages: nodePages, size: nodePageSize }) }}
          </th>
        </tr>
        <tr>
          <th class="frz">{{ t('grid.nodeRole') }}</th>
          <th>{{ t('grid.master') }}</th>
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
        <tr v-for="n in nodeRows" :key="n.name">
          <td class="frz">
            <div class="nodename">
              <span class="hdot" :class="n.master ? 'yellow' : 'green'"></span> {{ n.name }}
            </div>
            <div class="dim" style="font-size: 9.5px;">{{ n.role }}</div>
          </td>
          <td>{{ n.master ? '★' : '-' }}</td>
          <td class="num" :class="usageClass(n.heap, 85, 70)">{{ n.heap || '-' }}%</td>
          <td class="num" :class="usageClass(n.cpu, 80, 65)">{{ n.cpu || '-' }}%</td>
          <td class="num" :class="usageClass(n.disk, 80, 70)">{{ n.disk || '-' }}%</td>
          <td class="mono">{{ n.load1 || '-' }} / {{ n.load5 || '-' }} / {{ n.load15 || '-' }}</td>
          <td class="num mono">{{ n.shards ?? '-' }}</td>
          <td class="num mono dim">—</td>
          <td v-for="(cell, ci) in n.cells" :key="ci" class="node">
            <div v-if="cell && cell.length" class="shardcell">
              <span v-for="(s, si) in cell.slice(0, 8)" :key="si" class="shard" :class="[shardState(s), s.prirep === 'p' ? '' : 'rep']" :title="shardTip(s)">{{ s.shard }}</span>
            </div>
            <span v-else class="dim" style="font-size: 9px">·</span>
          </td>
        </tr>
      </tbody>
    </table>
  </template>
</template>
