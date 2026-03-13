<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiModal from '@/components/ui/UiModal.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import { useNetbirdStore } from '@/stores/netbird'
import type { NetBirdACLEdge, NetBirdACLNode, NetBirdMode } from '@/types/netbird'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const showNodesModal = ref(false)
const showEdgesModal = ref(false)
const showNotesModal = ref(false)

const netbirdStore = useNetbirdStore()

const graph = computed(() => netbirdStore.aclGraph.data)
const graphLoading = computed(() => netbirdStore.aclGraph.loading)
const graphError = computed(() => netbirdStore.aclGraph.error)

const allowEdges = computed(() => {
  if (!graph.value) return []
  return graph.value.edges.filter((edge) => {
    const action = edge.action.trim().toLowerCase()
    return action === 'accept' || action === 'allow'
  })
})

const modeLabel = (mode: NetBirdMode) => {
  if (mode === 'mode_a') return 'Mode A'
  if (mode === 'mode_b') return 'Mode B'
  return 'Legacy'
}

const defaultActionTone = (action: string): BadgeTone => {
  const normalized = action.trim().toLowerCase()
  if (normalized.includes('deny') || normalized.includes('block')) return 'ok'
  if (normalized.includes('allow') || normalized.includes('accept')) return 'warn'
  return 'neutral'
}

const nodeLabel = (node: { label: string; projectName?: string }) => {
  if (node.projectName) return node.projectName
  return node.label
}

const nodeKindLabel = (node: NetBirdACLNode) => {
  const normalizedKind = node.kind.trim().toLowerCase()
  if (normalizedKind === 'group') {
    const groupName = (node.groupName || node.label || '').trim().toLowerCase()
    if (groupName.includes('admin')) return 'Admins'
    return 'Group'
  }
  if (normalizedKind === 'service') {
    const label = (node.label || '').trim().toLowerCase()
    if (label.includes('panel')) return 'Panel'
    return 'Service'
  }
  if (normalizedKind === 'project') return 'Project'
  return node.kind || 'unknown'
}

const edgePortLabel = (edge: NetBirdACLEdge) => {
  if (edge.ports.length === 0) return 'any'
  return edge.ports.join(', ')
}

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
          Policy visibility and network flow
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
      <!-- Essential Info -->
      <UiPanel variant="soft" class="space-y-3 p-4">
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-sm font-medium text-[color:var(--text)]">Current Mode</span>
          <UiBadge tone="neutral">{{ modeLabel(graph.currentMode) }}</UiBadge>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-sm font-medium text-[color:var(--text)]">Default Action</span>
          <UiBadge :tone="defaultActionTone(graph.defaultAction)">
            {{ graph.defaultAction || 'unknown' }}
          </UiBadge>
        </UiListRow>
      </UiPanel>

      <!-- Interactive Cards -->
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
                {{ graph.nodes.length }}
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
                {{ allowEdges.length }}
              </p>
            </div>
            <NavIcon name="activity" class="h-6 w-6 text-[color:var(--muted)]" />
          </div>
        </UiPanel>

        <UiPanel
          variant="soft"
          :class="graph.notes.length > 0 ? 'border-[color:var(--warn)]' : ''"
          class="cursor-pointer p-4 transition hover:border-[color:var(--accent)] hover:shadow-sm"
          @click="showNotesModal = true"
        >
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Warnings
              </p>
              <p class="mt-2 text-2xl font-semibold text-[color:var(--text)]">
                {{ graph.notes.length }}
              </p>
            </div>
            <NavIcon name="activity" class="h-6 w-6" :class="graph.notes.length > 0 ? 'text-[color:var(--warn)]' : 'text-[color:var(--muted)]'" />
          </div>
        </UiPanel>
      </div>
    </div>
  </UiPanel>

  <!-- Nodes Modal -->
  <UiModal
    v-model="showNodesModal"
    title="Network Nodes"
    description="Groups, projects, and services in the ACL graph"
  >
    <div v-if="graph">
      <UiState v-if="graph.nodes.length === 0">
        No nodes were returned by the ACL graph endpoint.
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
              {{ nodeLabel(node) }}
            </p>
            <UiBadge tone="neutral">{{ nodeKindLabel(node) }}</UiBadge>
          </div>
          <p class="font-mono text-xs text-[color:var(--muted-2)]">
            {{ node.id }}
          </p>
        </UiListRow>
      </ul>
    </div>
  </UiModal>

  <!-- Edges Modal -->
  <UiModal
    v-model="showEdgesModal"
    title="Allow Edges"
    description="Permitted network flows between nodes"
  >
    <div v-if="graph">
      <UiState v-if="allowEdges.length === 0">
        No allow edges are currently defined for this mode.
      </UiState>
      <ul v-else class="space-y-3">
        <UiListRow
          v-for="edge in allowEdges"
          :key="edge.id"
          as="li"
          class="space-y-1 py-2"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="text-sm font-semibold text-[color:var(--text)]">
              {{ edge.from }} → {{ edge.to }}
            </p>
            <UiBadge tone="ok">{{ edge.action }}</UiBadge>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            Policy <span class="font-mono text-[color:var(--text)]">{{ edge.policy }}</span>,
            rule <span class="font-mono text-[color:var(--text)]">{{ edge.rule }}</span>,
            {{ edge.protocol.toUpperCase() }} ports {{ edgePortLabel(edge) }}
          </p>
        </UiListRow>
      </ul>
    </div>
  </UiModal>

  <!-- Notes/Warnings Modal -->
  <UiModal
    v-model="showNotesModal"
    title="Notes and Warnings"
    :description="graph && graph.notes.length > 0 ? `${graph.notes.length} warning(s) reported` : 'No warnings reported'"
  >
    <div v-if="graph">
      <UiState v-if="graph.notes.length === 0" tone="ok">
        No backend notes or warnings were reported.
      </UiState>
      <ul v-else class="space-y-2">
        <li
          v-for="(note, index) in graph.notes"
          :key="`netbird-acl-note-${index}`"
          class="rounded border border-[color:var(--border)] bg-[color:var(--surface)] px-3 py-2 text-sm text-[color:var(--muted)]"
        >
          {{ note }}
        </li>
      </ul>
    </div>
  </UiModal>
</template>
