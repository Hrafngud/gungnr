<script setup lang="ts">
import { computed, getCurrentInstance } from 'vue'
import type { NodeEdgeGraphEdge, NodeEdgeGraphNode, NodeEdgeGraphTone } from '@/types/graph'

interface RenderNode extends NodeEdgeGraphNode {
  x: number
  y: number
  width: number
  height: number
}

interface RenderEdge extends NodeEdgeGraphEdge {
  fromX: number
  fromY: number
  toX: number
  toY: number
  curve: number
}

const tonePalette: NodeEdgeGraphTone[] = [
  'neutral',
  'ok',
  'warn',
  'error',
  'inbound',
  'outbound',
  'allow',
  'running',
  'degraded',
  'failed',
  'missing',
  'unknown',
  'group',
  'service',
  'project',
]

const props = withDefaults(
  defineProps<{
    nodes: NodeEdgeGraphNode[]
    edges: NodeEdgeGraphEdge[]
    ariaLabel?: string
    focusNodeId?: string | null
    emptyMessage?: string
  }>(),
  {
    ariaLabel: 'Graph',
    focusNodeId: null,
    emptyMessage: 'No graph relationships are available.',
  },
)

const graphUid = getCurrentInstance()?.uid ?? 0

function normalizeTone(tone?: string): NodeEdgeGraphTone {
  if (!tone) return 'neutral'
  const normalized = tone.trim().toLowerCase() as NodeEdgeGraphTone
  if (tonePalette.includes(normalized)) return normalized
  return 'neutral'
}

function nodeWidth(label: string): number {
  return Math.min(168, Math.max(88, 28 + label.length * 6))
}

function ellipseOffset(width: number, height: number, unitX: number, unitY: number): number {
  const a = Math.max(1, width / 2)
  const b = Math.max(1, height / 2)
  return 1 / Math.sqrt((unitX * unitX) / (a * a) + (unitY * unitY) / (b * b))
}

function edgePath(edge: RenderEdge): string {
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

const toneMarkerIdMap = computed(() => {
  const ids: Record<NodeEdgeGraphTone, string> = {} as Record<NodeEdgeGraphTone, string>
  for (const tone of tonePalette) {
    ids[tone] = `node-edge-graph-arrow-${tone}-${graphUid}`
  }
  return ids
})

const layout = computed(() => {
  const validNodes = props.nodes
    .map((node) => ({
      ...node,
      id: node.id.trim(),
      label: node.label.trim(),
      subtitle: node.subtitle?.trim(),
      tone: normalizeTone(node.tone),
    }))
    .filter((node) => node.id && node.label)

  const nodeByID = new Map<string, (typeof validNodes)[number]>()
  for (const node of validNodes) {
    if (nodeByID.has(node.id)) continue
    nodeByID.set(node.id, node)
  }

  const validEdges = props.edges
    .map((edge) => ({
      ...edge,
      id: edge.id.trim(),
      from: edge.from.trim(),
      to: edge.to.trim(),
      label: edge.label?.trim(),
      tone: normalizeTone(edge.tone),
    }))
    .filter((edge) => edge.id && edge.from && edge.to && nodeByID.has(edge.from) && nodeByID.has(edge.to))

  const focusNodeID = props.focusNodeId?.trim() || ''
  const hasFocus = focusNodeID !== '' && nodeByID.has(focusNodeID)

  const visibleNodeIDs = new Set<string>()
  const visibleEdges = hasFocus
    ? validEdges.filter((edge) => edge.from === focusNodeID || edge.to === focusNodeID)
    : validEdges

  if (hasFocus) {
    visibleNodeIDs.add(focusNodeID)
    for (const edge of visibleEdges) {
      visibleNodeIDs.add(edge.from)
      visibleNodeIDs.add(edge.to)
    }
  } else {
    for (const node of validNodes) {
      visibleNodeIDs.add(node.id)
    }
  }

  const visibleNodes = validNodes.filter((node) => visibleNodeIDs.has(node.id))
  if (visibleNodes.length === 0) {
    return {
      width: 320,
      height: 190,
      nodes: [] as RenderNode[],
      edges: [] as RenderEdge[],
    }
  }

  const nodePositions = new Map<string, RenderNode>()
  const centerX = 184
  const centerY = hasFocus ? 114 : 104
  const outerPadding = 38
  const baseRadius = hasFocus
    ? Math.min(124, Math.max(66, 50 + (visibleNodes.length - 1) * 10))
    : Math.min(130, Math.max(64, 44 + visibleNodes.length * 11))

  if (hasFocus) {
    const focusNode = nodeByID.get(focusNodeID)
    if (focusNode) {
      nodePositions.set(focusNode.id, {
        ...focusNode,
        x: centerX,
        y: centerY,
        width: nodeWidth(focusNode.label) + 10,
        height: 34,
      })
    }

    const neighbors = visibleNodes.filter((node) => node.id !== focusNodeID)
    neighbors.forEach((node, index) => {
      const angle =
        neighbors.length === 1 ? -Math.PI / 2 : (-Math.PI / 2) + (index * (Math.PI * 2)) / neighbors.length
      nodePositions.set(node.id, {
        ...node,
        x: centerX + Math.cos(angle) * baseRadius,
        y: centerY + Math.sin(angle) * baseRadius,
        width: nodeWidth(node.label),
        height: 28,
      })
    })
  } else {
    visibleNodes.forEach((node, index) => {
      const angle = (-Math.PI / 2) + (index * (Math.PI * 2)) / visibleNodes.length
      nodePositions.set(node.id, {
        ...node,
        x: centerX + Math.cos(angle) * baseRadius,
        y: centerY + Math.sin(angle) * baseRadius,
        width: nodeWidth(node.label),
        height: 30,
      })
    })
  }

  const edgePairSet = new Set<string>()
  for (const edge of visibleEdges) {
    edgePairSet.add(`${edge.from}->${edge.to}`)
  }

  const renderEdges: RenderEdge[] = []
  for (const edge of visibleEdges) {
    const fromNode = nodePositions.get(edge.from)
    const toNode = nodePositions.get(edge.to)
    if (!fromNode || !toNode || fromNode.id === toNode.id) continue

    const dx = toNode.x - fromNode.x
    const dy = toNode.y - fromNode.y
    const distance = Math.hypot(dx, dy) || 1
    const unitX = dx / distance
    const unitY = dy / distance

    const fromOffset = ellipseOffset(fromNode.width, fromNode.height, unitX, unitY) + 1.5
    const toOffset = ellipseOffset(toNode.width, toNode.height, -unitX, -unitY) + 6
    const reciprocal = edgePairSet.has(`${edge.to}->${edge.from}`)
    const curve = reciprocal ? (edge.from.localeCompare(edge.to) < 0 ? 8 : -8) : 0

    renderEdges.push({
      ...edge,
      fromX: fromNode.x + unitX * fromOffset,
      fromY: fromNode.y + unitY * fromOffset,
      toX: toNode.x - unitX * toOffset,
      toY: toNode.y - unitY * toOffset,
      curve,
    })
  }

  const renderNodes = [...nodePositions.values()]
  const minX = Math.min(...renderNodes.map((node) => node.x - node.width / 2))
  const maxX = Math.max(...renderNodes.map((node) => node.x + node.width / 2))
  const minY = Math.min(...renderNodes.map((node) => node.y - node.height / 2))
  const maxY = Math.max(...renderNodes.map((node) => node.y + node.height / 2))

  const shiftX = outerPadding - minX
  const shiftY = outerPadding - minY
  for (const node of renderNodes) {
    node.x += shiftX
    node.y += shiftY
  }
  for (const edge of renderEdges) {
    edge.fromX += shiftX
    edge.toX += shiftX
    edge.fromY += shiftY
    edge.toY += shiftY
  }

  return {
    width: Math.max(290, maxX - minX + outerPadding * 2),
    height: Math.max(190, maxY - minY + outerPadding * 2),
    nodes: renderNodes,
    edges: renderEdges,
  }
})
</script>

<template>
  <div class="node-edge-graph">
    <svg
      v-if="layout.nodes.length > 0"
      class="node-edge-graph__canvas"
      role="img"
      :aria-label="ariaLabel"
      :viewBox="`0 0 ${layout.width} ${layout.height}`"
    >
      <defs>
        <marker
          v-for="tone in tonePalette"
          :id="toneMarkerIdMap[tone]"
          :key="`marker-${tone}`"
          viewBox="0 0 10 10"
          markerWidth="7"
          markerHeight="7"
          refX="8.5"
          refY="5"
          markerUnits="userSpaceOnUse"
          orient="auto-start-reverse"
        >
          <path :class="['node-edge-graph__arrow', `node-edge-graph__arrow--tone-${tone}`]" d="M0 0 L10 5 L0 10 z" />
        </marker>
      </defs>

      <path
        v-for="(edge, edgeIndex) in layout.edges"
        :key="edge.id"
        :d="edgePath(edge)"
        :class="['node-edge-graph__edge', `node-edge-graph__edge--tone-${edge.tone || 'neutral'}`]"
        :style="{ '--edge-delay': `${50 + edgeIndex * 35}ms` }"
        fill="none"
        :marker-end="`url(#${toneMarkerIdMap[normalizeTone(edge.tone)]})`"
      >
        <title>{{ edge.label || `${edge.from} -> ${edge.to}` }}</title>
      </path>

      <g
        v-for="(node, nodeIndex) in layout.nodes"
        :key="node.id"
        :class="['node-edge-graph__node', `node-edge-graph__node--tone-${node.tone || 'neutral'}`]"
        :style="{ '--node-delay': `${40 + nodeIndex * 30}ms` }"
      >
        <rect
          :x="node.x - node.width / 2"
          :y="node.y - node.height / 2"
          :width="node.width"
          :height="node.height"
          :rx="node.height / 2"
        />
        <text :x="node.x" :y="node.y">{{ node.label }}</text>
        <title>{{ node.subtitle ? `${node.label} • ${node.subtitle}` : node.label }}</title>
      </g>
    </svg>

    <p v-else class="node-edge-graph__empty">
      {{ emptyMessage }}
    </p>
  </div>
</template>

<style scoped>
.node-edge-graph {
  border-radius: 0.82rem;
  border: 1px solid color-mix(in srgb, var(--line) 80%, transparent);
  background:
    linear-gradient(to right, oklch(38% 0.012 254 / 0.42) 1px, transparent 1px),
    linear-gradient(to bottom, oklch(38% 0.012 254 / 0.42) 1px, transparent 1px),
    radial-gradient(circle at 1px 1px, oklch(80% 0.016 252 / 0.13) 1px, transparent 1.6px),
    linear-gradient(180deg, oklch(21% 0.014 252 / 0.42), oklch(18% 0.012 252 / 0.5));
  background-size: 14px 14px, 14px 14px, 14px 14px, 100% 100%;
  min-height: 12rem;
  display: grid;
  place-items: center;
  overflow: hidden;
  padding: 0.4rem;
}

.node-edge-graph__canvas {
  display: block;
  width: 100%;
  height: auto;
  max-height: 24rem;
}

.node-edge-graph__edge {
  stroke-width: 1.5;
  vector-effect: non-scaling-stroke;
  stroke-linecap: round;
  opacity: 0;
  animation: edge-enter 360ms var(--ease-out, cubic-bezier(0.22, 1, 0.36, 1)) forwards;
  animation-delay: var(--edge-delay, 0ms);
}

.node-edge-graph__node {
  opacity: 0;
  transform: translateY(3px) scale(0.98);
  transform-origin: center;
  animation: node-enter 320ms var(--ease-out, cubic-bezier(0.22, 1, 0.36, 1)) forwards;
  animation-delay: var(--node-delay, 0ms);
}

.node-edge-graph__node rect {
  stroke-width: 1;
  vector-effect: non-scaling-stroke;
}

.node-edge-graph__node text {
  fill: oklch(96% 0.01 252);
  font-size: 10px;
  font-weight: 640;
  text-anchor: middle;
  dominant-baseline: middle;
  pointer-events: none;
}

.node-edge-graph__edge--tone-neutral {
  stroke: oklch(76% 0.06 249 / 0.95);
}

.node-edge-graph__edge--tone-ok,
.node-edge-graph__edge--tone-allow {
  stroke: oklch(78% 0.14 147 / 0.95);
}

.node-edge-graph__edge--tone-warn {
  stroke: oklch(81% 0.13 86 / 0.95);
}

.node-edge-graph__edge--tone-error {
  stroke: oklch(74% 0.16 30 / 0.95);
}

.node-edge-graph__edge--tone-inbound {
  stroke: oklch(75% 0.11 196 / 0.95);
}

.node-edge-graph__edge--tone-outbound {
  stroke: oklch(74% 0.1 242 / 0.95);
}

.node-edge-graph__edge--tone-running {
  stroke: oklch(78% 0.14 147 / 0.95);
}

.node-edge-graph__edge--tone-degraded {
  stroke: oklch(81% 0.13 86 / 0.95);
}

.node-edge-graph__edge--tone-failed {
  stroke: oklch(74% 0.16 30 / 0.95);
}

.node-edge-graph__edge--tone-missing {
  stroke: oklch(80% 0.14 56 / 0.95);
}

.node-edge-graph__edge--tone-unknown {
  stroke: oklch(76% 0.03 252 / 0.95);
}

.node-edge-graph__edge--tone-group {
  stroke: oklch(80% 0.11 176 / 0.95);
}

.node-edge-graph__edge--tone-service {
  stroke: oklch(76% 0.08 242 / 0.95);
}

.node-edge-graph__edge--tone-project {
  stroke: oklch(82% 0.13 84 / 0.95);
}

.node-edge-graph__arrow {
  opacity: 0.98;
}

.node-edge-graph__arrow--tone-neutral {
  fill: oklch(76% 0.06 249);
}

.node-edge-graph__arrow--tone-ok,
.node-edge-graph__arrow--tone-allow {
  fill: oklch(78% 0.14 147);
}

.node-edge-graph__arrow--tone-warn {
  fill: oklch(81% 0.13 86);
}

.node-edge-graph__arrow--tone-error {
  fill: oklch(74% 0.16 30);
}

.node-edge-graph__arrow--tone-inbound {
  fill: oklch(75% 0.11 196);
}

.node-edge-graph__arrow--tone-outbound {
  fill: oklch(74% 0.1 242);
}

.node-edge-graph__arrow--tone-running {
  fill: oklch(78% 0.14 147);
}

.node-edge-graph__arrow--tone-degraded {
  fill: oklch(81% 0.13 86);
}

.node-edge-graph__arrow--tone-failed {
  fill: oklch(74% 0.16 30);
}

.node-edge-graph__arrow--tone-missing {
  fill: oklch(80% 0.14 56);
}

.node-edge-graph__arrow--tone-unknown {
  fill: oklch(76% 0.03 252);
}

.node-edge-graph__arrow--tone-group {
  fill: oklch(80% 0.11 176);
}

.node-edge-graph__arrow--tone-service {
  fill: oklch(76% 0.08 242);
}

.node-edge-graph__arrow--tone-project {
  fill: oklch(82% 0.13 84);
}

.node-edge-graph__node--tone-neutral rect {
  fill: oklch(48% 0.03 252 / 0.72);
  stroke: oklch(79% 0.05 252 / 0.9);
}

.node-edge-graph__node--tone-ok rect,
.node-edge-graph__node--tone-allow rect,
.node-edge-graph__node--tone-running rect {
  fill: oklch(48% 0.11 147 / 0.68);
  stroke: oklch(78% 0.15 147 / 0.9);
}

.node-edge-graph__node--tone-warn rect,
.node-edge-graph__node--tone-degraded rect {
  fill: oklch(54% 0.1 85 / 0.68);
  stroke: oklch(84% 0.14 88 / 0.9);
}

.node-edge-graph__node--tone-error rect,
.node-edge-graph__node--tone-failed rect {
  fill: oklch(50% 0.15 30 / 0.7);
  stroke: oklch(81% 0.17 30 / 0.92);
}

.node-edge-graph__node--tone-missing rect {
  fill: oklch(56% 0.12 58 / 0.68);
  stroke: oklch(84% 0.15 62 / 0.9);
}

.node-edge-graph__node--tone-unknown rect {
  fill: oklch(47% 0.03 252 / 0.68);
  stroke: oklch(80% 0.05 252 / 0.9);
}

.node-edge-graph__node--tone-inbound rect {
  fill: oklch(51% 0.09 198 / 0.7);
  stroke: oklch(80% 0.12 198 / 0.9);
}

.node-edge-graph__node--tone-outbound rect {
  fill: oklch(50% 0.08 241 / 0.7);
  stroke: oklch(79% 0.11 241 / 0.9);
}

.node-edge-graph__node--tone-group rect {
  fill: oklch(50% 0.1 176 / 0.7);
  stroke: oklch(82% 0.12 176 / 0.9);
}

.node-edge-graph__node--tone-service rect {
  fill: oklch(48% 0.07 242 / 0.7);
  stroke: oklch(79% 0.09 242 / 0.9);
}

.node-edge-graph__node--tone-project rect {
  fill: oklch(58% 0.1 88 / 0.68);
  stroke: oklch(85% 0.14 88 / 0.9);
}

.node-edge-graph__empty {
  margin: 0;
  min-height: 9rem;
  display: grid;
  place-items: center;
  text-align: center;
  padding: 0.8rem;
  font-size: 0.75rem;
  color: var(--muted);
}

@keyframes edge-enter {
  from {
    opacity: 0;
  }

  to {
    opacity: 0.95;
  }
}

@keyframes node-enter {
  from {
    opacity: 0;
    transform: translateY(3px) scale(0.98);
  }

  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}
</style>
