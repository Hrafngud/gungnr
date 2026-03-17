<script setup lang="ts">
import { computed } from 'vue'
import NodeEdgeGraph from '@/components/graph/NodeEdgeGraph.vue'
import type { NodeEdgeGraphEdge, NodeEdgeGraphNode } from '@/types/graph'
import type {
  WorkbenchDependencyGraph as WorkbenchDependencyGraphModel,
  WorkbenchDependencyNodeStatus,
} from '@/types/workbench'

const props = defineProps<{
  serviceName: string
  graph: WorkbenchDependencyGraphModel
}>()

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

function statusTone(status: WorkbenchDependencyNodeStatus): NodeEdgeGraphNode['tone'] {
  switch (status) {
    case 'running':
      return 'running'
    case 'degraded':
      return 'degraded'
    case 'failed':
      return 'failed'
    case 'missing':
      return 'missing'
    default:
      return 'unknown'
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

const connectedEdges = computed(() =>
  props.graph.edges.filter((edge) => {
    const from = edge.fromService.trim().toLowerCase()
    const to = edge.toService.trim().toLowerCase()
    return from === selectedServiceKey.value || to === selectedServiceKey.value
  }),
)

const graphNodes = computed<NodeEdgeGraphNode[]>(() => {
  const nodes = new Map<string, NodeEdgeGraphNode>()
  const selectedLabel = selectedServiceNode.value?.serviceName ?? props.serviceName.trim()
  const selectedStatus = selectedServiceNode.value?.status ?? 'unknown'
  const selectedStatusText = selectedServiceNode.value?.statusText ?? 'no runtime status'

  if (selectedLabel) {
    nodes.set(selectedLabel, {
      id: selectedLabel,
      label: selectedLabel,
      subtitle: `${statusLabel(selectedStatus)} · ${selectedStatusText}`,
      tone: statusTone(selectedStatus),
    })
  }

  for (const edge of connectedEdges.value) {
    const candidates = [edge.fromService, edge.toService]
    for (const candidate of candidates) {
      const key = candidate.trim().toLowerCase()
      if (!key) continue
      const node = nodeIndex.value.get(key)
      const label = node?.serviceName ?? candidate.trim()
      if (!label || nodes.has(label)) continue
      const status = node?.status ?? 'unknown'
      const statusText = node?.statusText ?? 'unknown'
      nodes.set(label, {
        id: label,
        label,
        subtitle: `${statusLabel(status)} · ${statusText}`,
        tone: statusTone(status),
      })
    }
  }

  const rows = [...nodes.values()]
  return rows.sort((left, right) => {
    if (left.id === selectedLabel) return -1
    if (right.id === selectedLabel) return 1
    return left.label.localeCompare(right.label)
  })
})

const graphEdges = computed<NodeEdgeGraphEdge[]>(() =>
  connectedEdges.value.map((edge) => {
    const fromKey = edge.fromService.trim().toLowerCase()
    const toKey = edge.toService.trim().toLowerCase()
    const fromService = nodeIndex.value.get(fromKey)?.serviceName ?? edge.fromService.trim()
    const toService = nodeIndex.value.get(toKey)?.serviceName ?? edge.toService.trim()
    let tone: NodeEdgeGraphEdge['tone'] = 'neutral'
    if (edge.failureSource) {
      tone = 'error'
    } else if (toService.toLowerCase() === selectedServiceKey.value) {
      tone = 'inbound'
    } else if (fromService.toLowerCase() === selectedServiceKey.value) {
      tone = 'outbound'
    }

    return {
      id: edge.key,
      from: fromService,
      to: toService,
      label: `${fromService} -> ${toService} (${statusLabel(edge.sourceStatus)})`,
      tone,
    }
  }),
)

const totalLinkedServices = computed(() => {
  const services = new Set<string>()
  for (const edge of connectedEdges.value) {
    const from = edge.fromService.trim().toLowerCase()
    const to = edge.toService.trim().toLowerCase()
    if (from && from !== selectedServiceKey.value) services.add(from)
    if (to && to !== selectedServiceKey.value) services.add(to)
  }
  return services.size
})

const failedDependencyCount = computed(() =>
  connectedEdges.value.filter((edge) => edge.failureSource).length,
)

const graphWarnings = computed(() =>
  props.graph.warnings
    .map((warning) => warning.trim())
    .filter((warning) => warning.length > 0),
)

const statusCountRows = computed(() => {
  const counts: Record<WorkbenchDependencyNodeStatus, number> = {
    running: 0,
    degraded: 0,
    failed: 0,
    missing: 0,
    unknown: 0,
  }

  for (const node of graphNodes.value) {
    if (node.id.trim().toLowerCase() === selectedServiceKey.value) continue
    const status = nodeIndex.value.get(node.id.trim().toLowerCase())?.status ?? 'unknown'
    counts[status] += 1
  }

  return (['running', 'degraded', 'failed', 'missing', 'unknown'] as WorkbenchDependencyNodeStatus[])
    .map((status) => ({
      status,
      count: counts[status],
      label: statusLabel(status),
    }))
    .filter((entry) => entry.count > 0)
})
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
      <NodeEdgeGraph
        v-if="graphEdges.length > 0"
        :nodes="graphNodes"
        :edges="graphEdges"
        :focus-node-id="selectedServiceNode?.serviceName ?? serviceName"
        :aria-label="`Dependency graph for ${serviceName}`"
        edge-flow
      />

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
  min-height: 12rem;
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
}
</style>
