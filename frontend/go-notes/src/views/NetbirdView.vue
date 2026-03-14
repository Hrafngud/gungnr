<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import NetbirdAclGraph from '@/components/netbird/NetbirdAclGraph.vue'
import NetbirdModeSwitchJobLog from '@/components/netbird/NetbirdModeSwitchJobLog.vue'
import NetbirdModeSwitcher from '@/components/netbird/NetbirdModeSwitcher.vue'
import NetbirdStatusCard from '@/components/netbird/NetbirdStatusCard.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import { useAuthStore } from '@/stores/auth'
import { useNetbirdStore } from '@/stores/netbird'
import { usePageLoadingStore } from '@/stores/pageLoading'

const authStore = useAuthStore()
const netbirdStore = useNetbirdStore()
const pageLoading = usePageLoadingStore()
const statusPollIntervalMs = 15000
let statusPollTimer: ReturnType<typeof setInterval> | null = null

const isAdmin = computed(() => authStore.isAdmin)
const accessLabel = computed(() => (isAdmin.value ? 'Admin controls enabled' : 'Read-only access'))
const roleLabel = computed(() => {
  if (authStore.isSuperUser) return 'SuperUser'
  if (authStore.isAdmin) return 'Admin'
  return 'User'
})

const loadPage = async () => {
  pageLoading.start('Loading NetBird control plane...')
  try {
    await Promise.all([netbirdStore.loadStatus(), netbirdStore.loadAclGraph()])
  } finally {
    pageLoading.stop()
  }
}

onMounted(() => {
  void loadPage()
  statusPollTimer = setInterval(() => {
    void netbirdStore.loadStatus()
  }, statusPollIntervalMs)
})

onBeforeUnmount(() => {
  if (statusPollTimer !== null) {
    clearInterval(statusPollTimer)
    statusPollTimer = null
  }
})
</script>

<template>
  <section class="page flex flex-col gap-2">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          NetBird
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Access policy control
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Review connectivity and ACL policy visibility. Mode switches remain admin-only.
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <UiBadge tone="neutral">
          Role: {{ roleLabel }}
        </UiBadge>
        <UiBadge :tone="isAdmin ? 'ok' : 'warn'">
          {{ accessLabel }}
        </UiBadge>
      </div>
    </div>

    <UiState v-if="!isAdmin" tone="warn">
      Read-only visibility is active. Mode planning and apply controls are limited to admin and superuser accounts.
    </UiState>
    <NetbirdStatusCard />
    <div class="grid w-full gap-4 lg:grid-cols-2">
      <div>
        <NetbirdModeSwitcher v-if="isAdmin" />
        <UiPanel v-else variant="soft" class="space-y-2 p-4">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Mode switcher
          </p>
          <p class="text-sm text-[color:var(--muted)]">
            This section is intentionally hidden for non-admin roles to prevent unauthorized mode changes.
          </p>
        </UiPanel>
      </div>
      <div>
        <NetbirdAclGraph />
      </div>
    </div>
    <NetbirdModeSwitchJobLog v-if="isAdmin" />
  </section>
</template>
