<script setup lang="ts">
import { computed, onMounted } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import { useNetbirdStore } from '@/stores/netbird'
import type { NetBirdACLEdge, NetBirdACLNode, NetBirdMode } from '@/types/netbird'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

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
          ACL graph
        </h2>
        <p class="mt-1 text-xs text-[color:var(--muted)]">
          Read-only policy visibility from managed ACL graph metadata.
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

    <UiState tone="neutral">
      Read-only visibility: mode changes and policy apply actions are handled elsewhere.
    </UiState>

    <UiState v-if="graphError" tone="error">
      {{ graphError }}
    </UiState>

    <UiPanel v-if="graphLoading && !graph" variant="soft" class="space-y-3 p-4">
      <UiSkeleton class="h-3 w-40" />
      <UiSkeleton class="h-3 w-full" />
      <UiSkeleton class="h-3 w-2/3" />
      <UiSkeleton class="h-3 w-3/4" />
    </UiPanel>

    <UiState v-else-if="!graph">
      ACL graph is not available yet.
    </UiState>

    <div v-else class="space-y-4">
      <div class="grid gap-4 md:grid-cols-3">
        <UiPanel variant="soft" class="space-y-2 p-3">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Current mode
          </p>
          <UiBadge tone="neutral">
            {{ modeLabel(graph.currentMode) }}
          </UiBadge>
          <p class="text-xs text-[color:var(--muted)]">
            Source: <span class="text-[color:var(--text)]">{{ graph.modeSource || 'n/a' }}</span>
          </p>
        </UiPanel>
        <UiPanel variant="soft" class="space-y-2 p-3">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Default action
          </p>
          <UiBadge :tone="defaultActionTone(graph.defaultAction)">
            {{ graph.defaultAction || 'unknown' }}
          </UiBadge>
          <p class="text-xs text-[color:var(--muted)]">
            Unspecified traffic paths follow this default action.
          </p>
        </UiPanel>
        <UiPanel variant="soft" class="space-y-2 p-3">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Graph summary
          </p>
          <div class="grid gap-1 text-xs text-[color:var(--muted)]">
            <p>Nodes: <span class="text-[color:var(--text)]">{{ graph.nodes.length }}</span></p>
            <p>Allow edges: <span class="text-[color:var(--text)]">{{ allowEdges.length }}</span></p>
            <p>Notes: <span class="text-[color:var(--text)]">{{ graph.notes.length }}</span></p>
          </div>
        </UiPanel>
      </div>

      <div class="grid gap-4 xl:grid-cols-2">
        <UiPanel variant="soft" class="space-y-3 p-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
              Nodes
            </p>
            <UiBadge tone="neutral">{{ graph.nodes.length }}</UiBadge>
          </div>
          <UiState v-if="graph.nodes.length === 0">
            No nodes were returned by the ACL graph endpoint.
          </UiState>
          <ul v-else class="space-y-2">
            <UiListRow
              v-for="node in graph.nodes"
              :key="node.id"
              as="li"
              class="space-y-1"
            >
              <div class="flex flex-wrap items-center justify-between gap-3">
                <p class="text-xs font-semibold text-[color:var(--text)]">
                  {{ nodeLabel(node) }}
                </p>
                <UiBadge tone="neutral">{{ nodeKindLabel(node) }}</UiBadge>
              </div>
              <p class="font-mono text-[11px] text-[color:var(--muted-2)]">
                {{ node.id }}
              </p>
            </UiListRow>
          </ul>
        </UiPanel>

        <UiPanel variant="soft" class="space-y-3 p-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
              Allow edges
            </p>
            <UiBadge tone="neutral">{{ allowEdges.length }}</UiBadge>
          </div>
          <UiState v-if="allowEdges.length === 0">
            No allow edges are currently defined for this mode.
          </UiState>
          <ul v-else class="space-y-2">
            <UiListRow
              v-for="edge in allowEdges"
              :key="edge.id"
              as="li"
              class="space-y-1"
            >
              <div class="flex flex-wrap items-center justify-between gap-3">
                <p class="text-xs font-semibold text-[color:var(--text)]">
                  {{ edge.from }} -> {{ edge.to }}
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
        </UiPanel>
      </div>

      <UiPanel variant="soft" class="space-y-3 p-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Notes and warnings
          </p>
          <UiBadge :tone="graph.notes.length > 0 ? 'warn' : 'ok'">
            {{ graph.notes.length }}
          </UiBadge>
        </div>
        <UiState v-if="graph.notes.length === 0" tone="ok">
          No backend notes or warnings were reported.
        </UiState>
        <ul v-else class="space-y-2 text-xs text-[color:var(--muted)]">
          <li
            v-for="(note, index) in graph.notes"
            :key="`netbird-acl-note-${index}`"
            class="rounded border border-[color:var(--border)] bg-[color:var(--surface)] px-3 py-2"
          >
            {{ note }}
          </li>
        </ul>
      </UiPanel>
    </div>
  </UiPanel>
</template>
