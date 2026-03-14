<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import NodeEdgeGraph from '@/components/graph/NodeEdgeGraph.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiModal from '@/components/ui/UiModal.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import type { NodeEdgeGraphEdge, NodeEdgeGraphNode } from '@/types/graph'
import { useNetbirdStore } from '@/stores/netbird'

const showNodesModal = ref(false)
const showEdgesModal = ref(false)

const netbirdStore = useNetbirdStore()

const graph = computed(() => netbirdStore.aclGraph.data)
const graphLoading = computed(() => netbirdStore.aclGraph.loading)
const graphError = computed(() => netbirdStore.aclGraph.error)

const graphNodes = computed<NodeEdgeGraphNode[]>(() => {
  if (!graph.value) return []
  return graph.value.nodes.map((node) => ({
    id: node.id,
    label: node.label,
    subtitle: node.kindLabel,
    tone: node.tone as NodeEdgeGraphNode['tone'],
  }))
})

const graphEdges = computed<NodeEdgeGraphEdge[]>(() => {
  if (!graph.value) return []
  return graph.value.edges.map((edge) => ({
    id: edge.id,
    from: edge.from,
    to: edge.to,
    label: edge.ruleLabel,
    tone: edge.tone as NodeEdgeGraphEdge['tone'],
  }))
})

const refreshAclGraph = async () => {
  await netbirdStore.loadAclGraph()
}

onMounted(() => {
  if (!graph.value) {
    void netbirdStore.loadAclGraph()
  }
})
</script>

<template>
  <UiPanel as="article" class="space-y-4 p-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          NetBird
        </p>
        <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
          ACL Graph
        </h2>
        <p class="mt-1 text-xs text-[color:var(--muted)]">
          Policy topology and permitted network flows.
        </p>
      </div>
      <UiButton variant="ghost" size="sm" :disabled="graphLoading" @click="refreshAclGraph">
        <span class="inline-flex items-center gap-2">
          <NavIcon name="refresh" class="h-3.5 w-3.5" />
          <UiInlineSpinner v-if="graphLoading" />
          Refresh
        </span>
      </UiButton>
    </div>

    <UiState v-if="graphError" tone="error">
      {{ graphError }}
    </UiState>

    <UiPanel v-if="graphLoading && !graph" variant="soft" class="space-y-3 p-4">
      <UiSkeleton class="h-3 w-40" />
      <UiSkeleton class="h-3 w-full" />
      <UiSkeleton class="h-3 w-2/3" />
    </UiPanel>

    <UiState v-else-if="!graph">
      ACL graph is not available yet.
    </UiState>

    <div v-else class="space-y-4">
      <div class="grid gap-3 sm:grid-cols-3">
        <UiPanel
          variant="soft"
          class="cursor-pointer p-4 transition hover:border-[color:var(--accent)] hover:shadow-sm"
          @click="showNodesModal = true"
        >
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Nodes
              </p>
              <p class="mt-2 text-2xl font-semibold text-[color:var(--text)]">
                {{ graph.summary.nodeCount }}
              </p>
            </div>
            <NavIcon name="network" class="h-6 w-6 text-[color:var(--muted)]" />
          </div>
        </UiPanel>

        <UiPanel
          variant="soft"
          class="cursor-pointer p-4 transition hover:border-[color:var(--accent)] hover:shadow-sm"
          @click="showEdgesModal = true"
        >
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Allow Edges
              </p>
              <p class="mt-2 text-2xl font-semibold text-[color:var(--text)]">
                {{ graph.summary.allowEdgeCount }}
              </p>
            </div>
            <NavIcon name="activity" class="h-6 w-6 text-[color:var(--muted)]" />
          </div>
        </UiPanel>

        <UiPanel variant="soft" class="p-4">
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Total Edges
          </p>
          <p class="mt-2 text-2xl font-semibold text-[color:var(--text)]">
            {{ graph.summary.edgeCount }}
          </p>
        </UiPanel>
      </div>

      <NodeEdgeGraph
        :nodes="graphNodes"
        :edges="graphEdges"
        aria-label="NetBird ACL topology graph"
        empty-message="No graph nodes were returned by the NetBird graph endpoint."
      />

      <UiState v-if="graph.notes.length > 0" tone="warn">
        {{ graph.notes[0] }}
      </UiState>
    </div>
  </UiPanel>

  <UiModal
    v-model="showNodesModal"
    title="Network Nodes"
    description="Groups, projects, and services in the NetBird graph"
  >
    <div v-if="graph">
      <UiState v-if="graph.nodes.length === 0">
        No nodes were returned by the NetBird graph endpoint.
      </UiState>
      <ul v-else class="space-y-3">
        <UiListRow
          v-for="node in graph.nodes"
          :key="node.id"
          as="li"
          class="space-y-1 py-2"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="text-sm font-semibold text-[color:var(--text)]">
              {{ node.label }}
            </p>
            <UiBadge tone="neutral">{{ node.kindLabel }}</UiBadge>
          </div>
          <p class="font-mono text-xs text-[color:var(--muted-2)]">
            {{ node.id }}
          </p>
        </UiListRow>
      </ul>
    </div>
  </UiModal>

  <UiModal
    v-model="showEdgesModal"
    title="Allow Edges"
    description="Permitted network flows between nodes"
  >
    <div v-if="graph">
      <UiState v-if="graph.edges.length === 0">
        No allow edges are currently defined for this mode.
      </UiState>
      <ul v-else class="space-y-3">
        <UiListRow
          v-for="edge in graph.edges"
          :key="edge.id"
          as="li"
          class="space-y-1 py-2"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="text-sm font-semibold text-[color:var(--text)]">
              {{ edge.fromLabel }} -> {{ edge.toLabel }}
            </p>
            <UiBadge tone="ok">{{ edge.action }}</UiBadge>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            {{ edge.ruleLabel }}
          </p>
        </UiListRow>
      </ul>
    </div>
  </UiModal>
</template>
