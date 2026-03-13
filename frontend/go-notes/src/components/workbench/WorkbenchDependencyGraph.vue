<script setup lang="ts">
import { computed, getCurrentInstance } from 'vue'
import type {
  WorkbenchDependencyGraph as WorkbenchDependencyGraphModel,
  WorkbenchDependencyNodeStatus,
} from '@/types/workbench'

type NodeRelation = 'upstream' | 'downstream' | 'bidirectional'

interface GraphNode {
  name: string
  relation: NodeRelation
  status: WorkbenchDependencyNodeStatus
  statusText: string
  x: number
  y: number
  width: number
  height: number
}

interface GraphEdge {
  key: string
  fromX: number
  fromY: number
  toX: number
  toY: number
  tone: 'inbound' | 'outbound'
  curve: number
  label: string
  failureSource: boolean
}

interface NeighborInventoryRow {
  name: string
  relation: NodeRelation
  status: WorkbenchDependencyNodeStatus
  statusText: string
  inboundFailure: boolean
  outboundFailure: boolean
}

const props = defineProps<{
  serviceName: string
  graph: WorkbenchDependencyGraphModel
}>()

const graphUid = getCurrentInstance()?.uid ?? 0
const inboundMarkerId = `dependency-graph-arrow-inbound-${graphUid}`
const outboundMarkerId = `dependency-graph-arrow-outbound-${graphUid}`
const failureMarkerId = `dependency-graph-arrow-failure-${graphUid}`

function truncateLabel(value: string, max = 14): string {
  if (value.length <= max) return value
  return `${value.slice(0, Math.max(1, max - 3))}...`
}

function nodeWidth(name: string): number {
  return Math.min(132, Math.max(76, 22 + name.length * 5))
}

function ellipseOffset(width: number, height: number, unitX: number, unitY: number): number {
  const a = Math.max(1, width / 2)
  const b = Math.max(1, height / 2)
  return 1 / Math.sqrt((unitX * unitX) / (a * a) + (unitY * unitY) / (b * b))
}

function edgePath(edge: GraphEdge): string {
  if (edge.curve === 0) {
    return `M ${edge.fromX} ${edge.fromY} L ${edge.toX} ${edge.toY}`
  }

  const dx = edge.toX - edge.fromX
  const dy = edge.toY - edge.fromY
  const distance = Math.hypot(dx, dy) || 1
  const controlX = (edge.fromX + edge.toX) / 2 + (-dy / distance) * edge.curve
  const controlY = (edge.fromY + edge.toY) / 2 + (dx / distance) * edge.curve

  return `M ${edge.fromX} ${edge.fromY} Q ${controlX} ${controlY} ${edge.toX} ${edge.toY}`
}

function statusClass(status: WorkbenchDependencyNodeStatus): string {
  switch (status) {
    case 'running':
      return 'dependency-graph__node--status-running'
    case 'degraded':
      return 'dependency-graph__node--status-degraded'
    case 'failed':
      return 'dependency-graph__node--status-failed'
    case 'missing':
      return 'dependency-graph__node--status-missing'
    default:
      return 'dependency-graph__node--status-unknown'
  }
}

function statusLabel(status: WorkbenchDependencyNodeStatus): string {
  switch (status) {
    case 'running':
      return 'Running'
    case 'degraded':
      return 'Degraded'
    case 'failed':
      return 'Failed'
    case 'missing':
      return 'Missing'
    default:
      return 'Unknown'
  }
}

const selectedServiceKey = computed(() => props.serviceName.trim().toLowerCase())

const nodeIndex = computed(() => {
  const index = new Map<string, WorkbenchDependencyGraphModel['nodes'][number]>()
  for (const node of props.graph.nodes) {
    const key = node.serviceName.trim().toLowerCase()
    if (!key || index.has(key)) continue
    index.set(key, node)
  }
  return index
})

const selectedServiceNode = computed(() => {
  if (!selectedServiceKey.value) return null
  return nodeIndex.value.get(selectedServiceKey.value) ?? null
})

const selectedServiceAvailable = computed(() => {
  if (selectedServiceNode.value) return true
  return props.graph.edges.some((edge) => {
    const from = edge.fromService.trim().toLowerCase()
    const to = edge.toService.trim().toLowerCase()
    return from === selectedServiceKey.value || to === selectedServiceKey.value
  })
})

const neighborInventory = computed<NeighborInventoryRow[]>(() => {
  if (!selectedServiceKey.value) return []

  const inventory = new Map<
    string,
    {
      name: string
      upstream: boolean
      downstream: boolean
      inboundFailure: boolean
      outboundFailure: boolean
      status: WorkbenchDependencyNodeStatus
      statusText: string
    }
  >()

  const ensureNeighbor = (serviceName: string) => {
    const key = serviceName.trim().toLowerCase()
    if (!key) return null

    const existing = inventory.get(key)
    if (existing) return existing

    const node = nodeIndex.value.get(key)
    const created = {
      name: node?.serviceName ?? serviceName.trim(),
      upstream: false,
      downstream: false,
      inboundFailure: false,
      outboundFailure: false,
      status: node?.status ?? 'unknown',
      statusText: node?.statusText ?? 'unknown',
    }
    inventory.set(key, created)
    return created
  }

  for (const edge of props.graph.edges) {
    const fromKey = edge.fromService.trim().toLowerCase()
    const toKey = edge.toService.trim().toLowerCase()

    if (toKey === selectedServiceKey.value) {
      const neighbor = ensureNeighbor(edge.fromService)
      if (!neighbor) continue
      neighbor.upstream = true
      neighbor.inboundFailure = neighbor.inboundFailure || edge.failureSource
      continue
    }

    if (fromKey === selectedServiceKey.value) {
      const neighbor = ensureNeighbor(edge.toService)
      if (!neighbor) continue
      neighbor.downstream = true
      neighbor.outboundFailure = neighbor.outboundFailure || edge.failureSource
    }
  }

  return [...inventory.values()]
    .map((entry) => {
      let relation: NodeRelation = 'upstream'
      if (entry.upstream && entry.downstream) relation = 'bidirectional'
      else if (entry.downstream) relation = 'downstream'

      return {
        name: entry.name,
        relation,
        status: entry.status,
        statusText: entry.statusText,
        inboundFailure: entry.inboundFailure,
        outboundFailure: entry.outboundFailure,
      }
    })
    .sort((a, b) => a.name.localeCompare(b.name))
})

const statusCountRows = computed(() => {
  const counts: Record<WorkbenchDependencyNodeStatus, number> = {
    running: 0,
    degraded: 0,
    failed: 0,
    missing: 0,
    unknown: 0,
  }

  for (const node of neighborInventory.value) {
    counts[node.status] += 1
  }

  return (['running', 'degraded', 'failed', 'missing', 'unknown'] as WorkbenchDependencyNodeStatus[])
    .map((status) => ({
      status,
      count: counts[status],
      label: statusLabel(status),
    }))
    .filter((entry) => entry.count > 0)
})

const layout = computed(() => {
  const neighborCount = neighborInventory.value.length
  const ringRadius = Math.min(108, Math.max(58, 48 + neighborCount * 10))
  const outerPadding = 24
  const centerX = ringRadius + outerPadding
  const centerY = ringRadius + outerPadding
  const width = centerX * 2
  const centerWidth = nodeWidth(props.serviceName) + 12
  const centerHeight = 31
  const centerStatus = selectedServiceNode.value?.status ?? 'unknown'
  const centerStatusText = selectedServiceNode.value?.statusText ?? 'unknown'

  const nodes: GraphNode[] = neighborInventory.value.map((neighbor, index) => {
    const angle = neighborCount === 1 ? -Math.PI / 2 : (-Math.PI / 2) + (index * (Math.PI * 2)) / neighborCount
    return {
      name: neighbor.name,
      relation: neighbor.relation,
      status: neighbor.status,
      statusText: neighbor.statusText,
      x: centerX + Math.cos(angle) * ringRadius,
      y: centerY + Math.sin(angle) * ringRadius,
      width: nodeWidth(neighbor.name),
      height: 26,
    }
  })

  const centerNode: GraphNode = {
    name: props.serviceName,
    relation: 'bidirectional',
    status: centerStatus,
    statusText: centerStatusText,
    x: centerX,
    y: centerY,
    width: centerWidth,
    height: centerHeight,
  }

  const edges: GraphEdge[] = []
  for (const node of nodes) {
    const deltaX = node.x - centerNode.x
    const deltaY = node.y - centerNode.y
    const distance = Math.hypot(deltaX, deltaY) || 1
    const unitX = deltaX / distance
    const unitY = deltaY / distance

    const fromCenterOffset = ellipseOffset(centerNode.width, centerNode.height, unitX, unitY) + 1.5
    const toCenterOffset = ellipseOffset(centerNode.width, centerNode.height, -unitX, -unitY) + 6
    const fromNodeOffset = ellipseOffset(node.width, node.height, -unitX, -unitY) + 1.5
    const toNodeOffset = ellipseOffset(node.width, node.height, unitX, unitY) + 6

    const inboundCurve = node.relation === 'bidirectional' ? 7 : 0
    const outboundCurve = node.relation === 'bidirectional' ? -7 : 0

    const relation = neighborInventory.value.find((entry) => entry.name === node.name)

    if (node.relation === 'upstream' || node.relation === 'bidirectional') {
      edges.push({
        key: `edge-inbound-${node.name}`,
        fromX: node.x - unitX * fromNodeOffset,
        fromY: node.y - unitY * fromNodeOffset,
        toX: centerNode.x + unitX * toCenterOffset,
        toY: centerNode.y + unitY * toCenterOffset,
        tone: 'inbound',
        curve: inboundCurve,
        label: `${props.serviceName} depends on ${node.name}`,
        failureSource: relation?.inboundFailure ?? false,
      })
    }

    if (node.relation === 'downstream' || node.relation === 'bidirectional') {
      edges.push({
        key: `edge-outbound-${node.name}`,
        fromX: centerNode.x + unitX * fromCenterOffset,
        fromY: centerNode.y + unitY * fromCenterOffset,
        toX: node.x - unitX * toNodeOffset,
        toY: node.y - unitY * toNodeOffset,
        tone: 'outbound',
        curve: outboundCurve,
        label: `${node.name} depends on ${props.serviceName}`,
        failureSource: relation?.outboundFailure ?? false,
      })
    }
  }

  const verticalMargin = 14
  const minY = Math.min(centerNode.y - centerNode.height / 2, ...nodes.map((node) => node.y - node.height / 2))
  const maxY = Math.max(centerNode.y + centerNode.height / 2, ...nodes.map((node) => node.y + node.height / 2))
  const yShift = verticalMargin - minY

  centerNode.y += yShift
  for (const node of nodes) {
    node.y += yShift
  }
  for (const edge of edges) {
    edge.fromY += yShift
    edge.toY += yShift
  }

  const height = maxY - minY + verticalMargin * 2

  return {
    width,
    height,
    centerX,
    centerY,
    centerWidth,
    centerHeight,
    centerNode,
    nodes,
    edges,
  }
})

const totalLinkedServices = computed(() => neighborInventory.value.length)
const failedDependencyCount = computed(() => layout.value.edges.filter((edge) => edge.failureSource).length)
const graphWarnings = computed(() =>
  props.graph.warnings
    .map((warning) => warning.trim())
    .filter((warning) => warning.length > 0),
)
</script>

<template>
  <div class="dependency-graph">
    <div class="dependency-graph__meta">
      <p class="dependency-graph__kicker">Topology View</p>
      <p class="dependency-graph__summary">
        {{ totalLinkedServices }} linked service{{ totalLinkedServices === 1 ? '' : 's' }}
      </p>
      <p class="dependency-graph__summary dependency-graph__summary--failure">
        {{ failedDependencyCount }} failure-sourced edge{{ failedDependencyCount === 1 ? '' : 's' }}
      </p>
      <p
        v-if="graphWarnings.length > 0"
        class="dependency-graph__summary dependency-graph__summary--warning"
      >
        {{ graphWarnings[0] }}
      </p>
      <ul class="dependency-graph__legend">
        <li>
          <span class="dependency-graph__legend-line dependency-graph__legend-line--inbound" />
          This service depends on
        </li>
        <li>
          <span class="dependency-graph__legend-line dependency-graph__legend-line--outbound" />
          Depends on this service
        </li>
        <li>
          <span class="dependency-graph__legend-line dependency-graph__legend-line--failure" />
          Dependency source failing
        </li>
      </ul>
      <ul v-if="statusCountRows.length > 0" class="dependency-graph__status-legend">
        <li v-for="row in statusCountRows" :key="`status-${row.status}`">
          <span :class="['dependency-graph__status-dot', `dependency-graph__status-dot--${row.status}`]" />
          {{ row.label }} ({{ row.count }})
        </li>
      </ul>
    </div>

    <div class="dependency-graph__surface">
      <svg
        v-if="layout.nodes.length > 0"
        class="dependency-graph__canvas"
        role="img"
        :aria-label="`Dependency graph for ${serviceName}`"
        :viewBox="`0 0 ${layout.width} ${layout.height}`"
      >
        <defs>
          <marker
            :id="inboundMarkerId"
            viewBox="0 0 10 10"
            markerWidth="7.2"
            markerHeight="7.2"
            refX="8.5"
            refY="5"
            markerUnits="userSpaceOnUse"
            orient="auto-start-reverse"
          >
            <path d="M0 0 L10 5 L0 10 z" class="dependency-graph__arrow dependency-graph__arrow--inbound" />
          </marker>
          <marker
            :id="outboundMarkerId"
            viewBox="0 0 10 10"
            markerWidth="7.2"
            markerHeight="7.2"
            refX="8.5"
            refY="5"
            markerUnits="userSpaceOnUse"
            orient="auto-start-reverse"
          >
            <path d="M0 0 L10 5 L0 10 z" class="dependency-graph__arrow dependency-graph__arrow--outbound" />
          </marker>
          <marker
            :id="failureMarkerId"
            viewBox="0 0 10 10"
            markerWidth="7.2"
            markerHeight="7.2"
            refX="8.5"
            refY="5"
            markerUnits="userSpaceOnUse"
            orient="auto-start-reverse"
          >
            <path d="M0 0 L10 5 L0 10 z" class="dependency-graph__arrow dependency-graph__arrow--failure" />
          </marker>
        </defs>

        <path
          v-for="(edge, edgeIndex) in layout.edges"
          :key="edge.key"
          :d="edgePath(edge)"
          :class="[
            'dependency-graph__edge',
            edge.failureSource
              ? 'dependency-graph__edge--failure'
              : edge.tone === 'inbound'
                ? 'dependency-graph__edge--inbound'
                : 'dependency-graph__edge--outbound',
          ]"
          :style="{ '--edge-delay': `${50 + edgeIndex * 50}ms` }"
          fill="none"
          :marker-end="
            edge.failureSource
              ? `url(#${failureMarkerId})`
              : edge.tone === 'inbound'
                ? `url(#${inboundMarkerId})`
                : `url(#${outboundMarkerId})`
          "
        >
          <title>{{ edge.label }}</title>
        </path>

        <g
          v-for="(node, nodeIndex) in layout.nodes"
          :key="`node-${node.name}`"
          :class="[
            'dependency-graph__node',
            statusClass(node.status),
            node.relation === 'upstream' ? 'dependency-graph__node--upstream' : '',
            node.relation === 'downstream' ? 'dependency-graph__node--downstream' : '',
            node.relation === 'bidirectional' ? 'dependency-graph__node--bidirectional' : '',
          ]"
          :style="{ '--node-delay': `${80 + nodeIndex * 45}ms` }"
        >
          <rect
            :x="node.x - node.width / 2"
            :y="node.y - node.height / 2"
            :width="node.width"
            :height="node.height"
            :rx="node.height / 2"
          />
          <text :x="node.x" :y="node.y">
            {{ truncateLabel(node.name) }}
          </text>
          <title>{{ `${node.name} • ${statusLabel(node.status)} • ${node.statusText}` }}</title>
        </g>

        <g
          :class="[
            'dependency-graph__node',
            'dependency-graph__node--active',
            statusClass(layout.centerNode.status),
          ]"
          style="--node-delay: 35ms;"
        >
          <rect
            :x="layout.centerX - layout.centerWidth / 2"
            :y="layout.centerY - layout.centerHeight / 2"
            :width="layout.centerWidth"
            :height="layout.centerHeight"
            :rx="layout.centerHeight / 2"
          />
          <text :x="layout.centerX" :y="layout.centerY">
            {{ truncateLabel(serviceName, 16) }}
          </text>
          <title>{{ `${serviceName} • ${statusLabel(layout.centerNode.status)} • ${layout.centerNode.statusText}` }}</title>
        </g>
      </svg>

      <p v-else-if="selectedServiceAvailable" class="dependency-graph__empty">
        No upstream or downstream links are registered for this service yet.
      </p>
      <p v-else class="dependency-graph__empty">
        This service is not present in the current dependency graph payload.
      </p>
    </div>
  </div>
</template>

<style scoped>
.dependency-graph {
  --ease-out-quart: cubic-bezier(0.25, 1, 0.5, 1);
  --ease-out-quint: cubic-bezier(0.22, 1, 0.36, 1);
  --ease-out-expo: cubic-bezier(0.16, 1, 0.3, 1);
  display: grid;
  gap: 0.55rem;
  padding: 0.62rem;
  border-radius: 0.92rem;
  border: 1px solid color-mix(in srgb, var(--line) 86%, transparent);
  background: color-mix(in srgb, var(--panel) 78%, transparent);
}

.dependency-graph__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.2rem 0.9rem;
}

.dependency-graph__kicker {
  margin: 0;
  font-size: 0.63rem;
  letter-spacing: 0.19em;
  text-transform: uppercase;
  color: var(--muted-2);
}

.dependency-graph__summary {
  margin: 0;
  font-size: 0.71rem;
  color: color-mix(in srgb, var(--muted) 76%, oklch(83% 0.03 250));
}

.dependency-graph__summary--failure {
  color: oklch(74% 0.16 30);
}

.dependency-graph__summary--warning {
  color: oklch(82% 0.12 82);
}

.dependency-graph__legend,
.dependency-graph__status-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.24rem 0.7rem;
  padding: 0;
  margin: 0 0 0 auto;
  list-style: none;
  font-size: 0.67rem;
  color: color-mix(in srgb, var(--muted) 72%, oklch(78% 0.03 250));
}

.dependency-graph__legend li,
.dependency-graph__status-legend li {
  display: inline-flex;
  align-items: center;
  gap: 0.36rem;
}

.dependency-graph__legend-line {
  width: 1.1rem;
  height: 2px;
  border-radius: 999px;
}

.dependency-graph__legend-line--inbound {
  background: oklch(75% 0.11 196 / 0.95);
}

.dependency-graph__legend-line--outbound {
  background: oklch(74% 0.1 242 / 0.95);
}

.dependency-graph__legend-line--failure {
  background: oklch(74% 0.16 30 / 0.95);
}

.dependency-graph__status-dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 999px;
  display: inline-block;
}

.dependency-graph__status-dot--running {
  background: oklch(76% 0.15 147);
}

.dependency-graph__status-dot--degraded {
  background: oklch(80% 0.14 85);
}

.dependency-graph__status-dot--failed {
  background: oklch(74% 0.16 30);
}

.dependency-graph__status-dot--missing {
  background: oklch(79% 0.15 58);
}

.dependency-graph__status-dot--unknown {
  background: oklch(76% 0.03 255);
}

.dependency-graph__surface {
  border-radius: 0.78rem;
  border: 1px solid color-mix(in srgb, var(--line) 82%, transparent);
  background:
    linear-gradient(to right, oklch(38% 0.012 254 / 0.46) 1px, transparent 1px),
    linear-gradient(to bottom, oklch(38% 0.012 254 / 0.46) 1px, transparent 1px),
    radial-gradient(circle at 1px 1px, oklch(79% 0.014 250 / 0.12) 1px, transparent 1.6px),
    linear-gradient(180deg, oklch(22% 0.015 252 / 0.36), oklch(19% 0.013 252 / 0.42));
  background-size: 14px 14px, 14px 14px, 14px 14px, 100% 100%;
  min-height: 9rem;
  display: grid;
  place-items: center;
  padding: 0.35rem 0.52rem;
}

.dependency-graph__canvas {
  display: block;
  width: min(100%, 360px);
  height: auto;
  max-height: 14.5rem;
}

.dependency-graph__arrow {
  opacity: 0.98;
}

.dependency-graph__arrow--inbound {
  fill: oklch(75% 0.11 196);
}

.dependency-graph__arrow--outbound {
  fill: oklch(74% 0.1 242);
}

.dependency-graph__arrow--failure {
  fill: oklch(74% 0.16 30);
}

.dependency-graph__edge {
  stroke-width: 1.5;
  vector-effect: non-scaling-stroke;
  stroke-linecap: round;
  opacity: 0.96;
}

.dependency-graph__edge--inbound {
  stroke: oklch(75% 0.11 196 / 0.95);
}

.dependency-graph__edge--outbound {
  stroke: oklch(74% 0.1 242 / 0.95);
}

.dependency-graph__edge--failure {
  stroke: oklch(74% 0.16 30 / 0.95);
}

.dependency-graph__node {
  transform-box: fill-box;
  transform-origin: center;
}

.dependency-graph__node rect {
  stroke-width: 1;
  vector-effect: non-scaling-stroke;
}

.dependency-graph__node text {
  fill: oklch(97% 0.013 252);
  font-size: 9.5px;
  font-weight: 650;
  text-anchor: middle;
  dominant-baseline: middle;
  pointer-events: none;
}

.dependency-graph__node--status-running rect {
  fill: oklch(48% 0.11 147 / 0.68);
  stroke: oklch(78% 0.15 147 / 0.9);
}

.dependency-graph__node--status-degraded rect {
  fill: oklch(53% 0.11 85 / 0.68);
  stroke: oklch(84% 0.14 88 / 0.9);
}

.dependency-graph__node--status-failed rect {
  fill: oklch(50% 0.15 30 / 0.7);
  stroke: oklch(81% 0.17 30 / 0.92);
}

.dependency-graph__node--status-missing rect {
  fill: oklch(55% 0.12 58 / 0.66);
  stroke: oklch(84% 0.15 62 / 0.9);
}

.dependency-graph__node--status-unknown rect {
  fill: oklch(48% 0.03 252 / 0.68);
  stroke: oklch(80% 0.05 252 / 0.9);
}

.dependency-graph__node--active rect {
  stroke-width: 1.45;
}

.dependency-graph__node--upstream rect,
.dependency-graph__node--downstream rect,
.dependency-graph__node--bidirectional rect {
  filter: saturate(1.05);
}

.dependency-graph__empty {
  margin: 0;
  min-height: 9rem;
  display: grid;
  place-items: center;
  text-align: center;
  padding: 0.8rem;
  font-size: 0.72rem;
  color: var(--muted);
}

@media (max-width: 780px) {
  .dependency-graph {
    padding: 0.55rem;
  }

  .dependency-graph__legend,
  .dependency-graph__status-legend {
    margin-left: 0;
    width: 100%;
  }

  .dependency-graph__canvas {
    width: min(100%, 312px);
  }
}

@media (prefers-reduced-motion: no-preference) {
  .dependency-graph__edge {
    stroke-dasharray: 3 5;
    animation:
      dependency-edge-appear 520ms var(--ease-out-quint) both,
      dependency-edge-flow 10s linear infinite;
    animation-delay: var(--edge-delay, 0ms), calc(var(--edge-delay, 0ms) + 420ms);
  }

  .dependency-graph__edge--failure {
    stroke-dasharray: 2 4;
  }

  .dependency-graph__node {
    opacity: 0;
    animation: dependency-node-enter 560ms var(--ease-out-expo) both;
    animation-delay: var(--node-delay, 0ms);
  }

  .dependency-graph__node--active {
    animation: dependency-node-enter 560ms var(--ease-out-expo) both;
    animation-delay: var(--node-delay, 0ms);
  }

  @keyframes dependency-edge-appear {
    from {
      opacity: 0;
      stroke-dashoffset: 24;
    }
    to {
      opacity: 0.96;
      stroke-dashoffset: 0;
    }
  }

  @keyframes dependency-edge-flow {
    to {
      stroke-dashoffset: -56;
    }
  }

  @keyframes dependency-node-enter {
    from {
      opacity: 0;
      transform: translateY(8px) scale(0.96);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }
}

@media (prefers-reduced-motion: reduce) {
  .dependency-graph__edge,
  .dependency-graph__node,
  .dependency-graph__node--active {
    animation: none !important;
  }
}
</style>
