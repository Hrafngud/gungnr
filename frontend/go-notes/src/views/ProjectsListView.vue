<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import { useProjectsStore } from '@/stores/projects'
import { usePageLoadingStore } from '@/stores/pageLoading'
import type { Project } from '@/types/projects'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const projectsStore = useProjectsStore()
const pageLoading = usePageLoadingStore()
const searchQuery = ref('')
const currentPage = ref(1)
const pageSize = 9

const normalizeStatus = (status: string) => status.trim().toLowerCase()

const isHealthyStatus = (status: string) => {
  const normalized = normalizeStatus(status)
  if (!normalized) return false
  if (normalized.includes('unhealthy')) return false
  return (
    normalized === 'running' ||
    normalized === 'up' ||
    normalized === 'healthy' ||
    normalized.includes('running') ||
    normalized.includes('healthy')
  )
}

const isDownStatus = (status: string) => {
  const normalized = normalizeStatus(status)
  if (!normalized) return false
  return (
    normalized === 'down' ||
    normalized.includes('stopped') ||
    normalized.includes('exited') ||
    normalized.includes('failed') ||
    normalized.includes('error')
  )
}

const isDegradedStatus = (status: string) => {
  const normalized = normalizeStatus(status)
  if (!normalized || isHealthyStatus(normalized) || isDownStatus(normalized)) return false
  return (
    normalized.includes('degraded') ||
    normalized.includes('partial') ||
    normalized.includes('starting') ||
    normalized.includes('unhealthy')
  )
}

const projectTone = (project: Project): BadgeTone => {
  const normalized = normalizeStatus(project.status)
  if (!normalized) return 'neutral'
  if (isDownStatus(normalized)) return 'error'
  if (isDegradedStatus(normalized)) return 'warn'
  if (isHealthyStatus(normalized)) return 'ok'
  if (normalized.includes('building') || normalized.includes('pending')) return 'warn'
  return 'neutral'
}

const fmtDate = (value: string) => {
  if (!value) return '—'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString()
}

const filteredProjects = computed(() => {
  const needle = searchQuery.value.trim().toLowerCase()
  if (!needle) return projectsStore.projects
  return projectsStore.projects.filter((project) => {
    const haystack = [
      project.name,
      project.status,
      project.path,
      project.repoUrl,
      String(project.proxyPort || ''),
      String(project.dbPort || ''),
    ]
      .join(' ')
      .toLowerCase()
    return haystack.includes(needle)
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredProjects.value.length / pageSize)))

const paginatedProjects = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  const end = start + pageSize
  return filteredProjects.value.slice(start, end)
})

const canGoBack = computed(() => currentPage.value > 1)
const canGoForward = computed(() => currentPage.value < totalPages.value)
const visiblePageNumbers = computed(() => {
  const maxButtons = 5
  if (totalPages.value <= maxButtons) {
    return Array.from({ length: totalPages.value }, (_, index) => index + 1)
  }

  let start = Math.max(1, currentPage.value - Math.floor(maxButtons / 2))
  let end = start + maxButtons - 1
  if (end > totalPages.value) {
    end = totalPages.value
    start = end - maxButtons + 1
  }

  return Array.from({ length: end - start + 1 }, (_, index) => start + index)
})

const pageSummary = computed(() => {
  if (filteredProjects.value.length === 0) return '0 projects'
  const start = (currentPage.value - 1) * pageSize + 1
  const end = Math.min(currentPage.value * pageSize, filteredProjects.value.length)
  return `${start}-${end} of ${filteredProjects.value.length} projects`
})

const healthyCount = computed(() =>
  projectsStore.projects.filter((project) => projectTone(project) === 'ok').length,
)

const goToPage = (nextPage: number) => {
  if (nextPage < 1 || nextPage > totalPages.value) return
  currentPage.value = nextPage
}

const load = async () => {
  pageLoading.start('Loading projects...')
  await projectsStore.fetchProjects()
  pageLoading.stop()
}

watch(searchQuery, () => {
  currentPage.value = 1
})

watch(filteredProjects, () => {
  if (currentPage.value > totalPages.value) {
    currentPage.value = totalPages.value
  }
})

onMounted(load)
</script>

<template>
  <section class="page space-y-8">
    <header class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Projects</p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">Workspace</h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Browse deployed projects, inspect runtime metadata, and open project-specific controls.
        </p>
      </div>
      <UiButton variant="ghost" size="sm" @click="load">
        <span class="inline-flex items-center gap-2">
          <NavIcon name="refresh" class="h-3.5 w-3.5" />
          Refresh
        </span>
      </UiButton>
    </header>

    <div class="grid gap-3 sm:grid-cols-3">
      <UiPanel variant="soft" class="space-y-2 p-4">
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Total</p>
        <p class="text-lg font-semibold">{{ projectsStore.projects.length }}</p>
      </UiPanel>
      <UiPanel variant="soft" class="space-y-2 p-4">
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Healthy</p>
        <p class="text-lg font-semibold text-[color:var(--success)]">{{ healthyCount }}</p>
      </UiPanel>
      <UiPanel variant="soft" class="space-y-2 p-4">
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Filtered</p>
        <p class="text-lg font-semibold">{{ filteredProjects.length }}</p>
      </UiPanel>
    </div>

    <UiPanel class="space-y-5 p-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="w-full max-w-xl space-y-2">
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Filter</p>
          <UiInput
            v-model="searchQuery"
            class="min-w-[220px]"
            placeholder="Filter by name, status, repo, path, or port"
          />
        </div>
        <UiState :loading="projectsStore.loading" class="text-xs">
          {{ projectsStore.loading ? 'Refreshing project list...' : 'Project list ready' }}
        </UiState>
      </div>

      <hr />

      <UiState v-if="projectsStore.error" tone="error">{{ projectsStore.error }}</UiState>
      <UiState v-else-if="!projectsStore.loading && filteredProjects.length === 0">
        No projects matched the current filter.
      </UiState>

      <div v-else class="space-y-4">
        <div class="stagger grid gap-3 sm:grid-cols-2 2xl:grid-cols-3">
          <RouterLink
            v-for="project in paginatedProjects"
            :key="project.id"
            :to="`/projects/${encodeURIComponent(project.name)}`"
            class="project-card-link"
          >
            <UiPanel variant="soft" class="project-card h-full p-4">
              <div class="flex items-start justify-between gap-2">
                <div class="space-y-1">
                  <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project</p>
                  <h2 class="text-lg font-semibold text-[color:var(--text)]">{{ project.name }}</h2>
                </div>
                <UiBadge :tone="projectTone(project)">
                  {{ project.status || 'unknown' }}
                </UiBadge>
              </div>

              <p class="project-path mt-3 min-h-[2.5rem] text-sm leading-6 text-[color:var(--muted)]">
                {{ project.path || 'Path unavailable' }}
              </p>

              <UiPanel variant="raise" class="mt-4 grid grid-cols-2 gap-2 p-3 text-xs">
                <div class="space-y-1">
                  <p class="uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Proxy</p>
                  <p class="font-semibold text-[color:var(--text)]">{{ project.proxyPort || '—' }}</p>
                </div>
                <div class="space-y-1">
                  <p class="uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DB</p>
                  <p class="font-semibold text-[color:var(--text)]">{{ project.dbPort || '—' }}</p>
                </div>
              </UiPanel>

              <div class="mt-4 flex items-center justify-between text-xs text-[color:var(--muted)]">
                <span>{{ fmtDate(project.updatedAt) }}</span>
                <span class="inline-flex items-center gap-1 text-[color:var(--accent-ink)]">
                  Open
                  <NavIcon name="arrow-right" class="h-3.5 w-3.5" />
                </span>
              </div>
            </UiPanel>
          </RouterLink>
        </div>

        <div
          v-if="totalPages > 1"
          class="flex flex-wrap items-center justify-between gap-3 bg-[color:var(--surface-2)] px-4 py-3 text-xs text-[color:var(--muted)]"
        >
          <div class="flex items-center gap-2">
            <span class="text-[color:var(--text)]">Page {{ currentPage }} of {{ totalPages }}</span>
            <span>{{ pageSummary }}</span>
          </div>
          <div class="flex items-center gap-2">
            <UiButton
              variant="ghost"
              size="sm"
              :disabled="projectsStore.loading || !canGoBack"
              @click="goToPage(currentPage - 1)"
            >
              <span class="flex items-center gap-2">
                <NavIcon name="arrow-left" class="h-3.5 w-3.5" />
                Previous
              </span>
            </UiButton>
            <UiButton
              v-for="page in visiblePageNumbers"
              :key="page"
              variant="ghost"
              size="sm"
              class="hidden min-w-8 justify-center sm:inline-flex"
              :class="page === currentPage ? 'border-[color:var(--accent)] text-[color:var(--accent-ink)]' : ''"
              :disabled="projectsStore.loading"
              @click="goToPage(page)"
            >
              {{ page }}
            </UiButton>
            <UiButton
              variant="ghost"
              size="sm"
              :disabled="projectsStore.loading || !canGoForward"
              @click="goToPage(currentPage + 1)"
            >
              <span class="flex items-center gap-2">
                Next
                <NavIcon name="arrow-right" class="h-3.5 w-3.5" />
              </span>
            </UiButton>
          </div>
        </div>
      </div>
    </UiPanel>
  </section>
</template>

<style scoped>
.project-path {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.project-card-link {
  display: block;
  border-radius: 5px;
}

.project-card {
  transition: background-color 0.2s ease, transform 0.2s ease;
}

.project-card-link:hover .project-card,
.project-card-link:focus-visible .project-card {
  background: var(--surface-3);
  transform: translateY(-2px);
}

.project-card-link:focus-visible {
  outline: none;
}
</style>
