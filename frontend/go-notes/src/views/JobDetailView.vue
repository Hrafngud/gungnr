<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import { jobsApi } from '@/services/jobs'
import { apiErrorMessage, getApiBaseUrl } from '@/services/api'
import { jobStatusLabel, jobStatusTone } from '@/utils/jobStatus'
import { usePageLoadingStore } from '@/stores/pageLoading'
import type { JobDetail } from '@/types/jobs'

const route = useRoute()
const job = ref<JobDetail | null>(null)
const logLines = ref<string[]>([])
const error = ref<string | null>(null)
const streaming = ref(false)
const connected = ref(false)
const pageLoading = usePageLoadingStore()
let source: EventSource | null = null

const jobId = Number(route.params.id)

const fetchJob = async () => {
  if (!Number.isFinite(jobId)) {
    error.value = 'Invalid job id.'
    return
  }
  try {
    const { data } = await jobsApi.get(jobId)
    job.value = data
    logLines.value = data.logLines ?? []
  } catch (err) {
    error.value = apiErrorMessage(err)
  }
}

const startStream = (offset = 0) => {
  if (!Number.isFinite(jobId)) return
  const base = getApiBaseUrl().replace(/\/$/, '')
  const url = `${base}/api/v1/jobs/${jobId}/stream?offset=${offset}`
  streaming.value = true
  source = new EventSource(url, { withCredentials: true })

  source.addEventListener('log', (event) => {
    try {
      const payload = JSON.parse((event as MessageEvent<string>).data)
      if (payload?.line) {
        logLines.value.push(payload.line)
      }
    } catch {
      // ignore malformed entries
    }
  })

  source.addEventListener('done', (event) => {
    try {
      const payload = JSON.parse((event as MessageEvent<string>).data)
      if (job.value && payload?.status) {
        job.value.status = payload.status
      }
    } catch {
      // ignore malformed entries
    }
    source?.close()
    streaming.value = false
    connected.value = false
  })

  source.addEventListener('error', () => {
    connected.value = false
    streaming.value = false
  })

  source.onopen = () => {
    connected.value = true
  }
}

onMounted(async () => {
  pageLoading.start('Loading job details...')
  await fetchJob()
  if (job.value && job.value.status !== 'completed' && job.value.status !== 'failed') {
    const seed = logLines.value.join('\n')
    const offset = seed.length > 0 ? seed.length + 1 : 0
    startStream(offset)
  }
  pageLoading.stop()
})

onBeforeUnmount(() => {
  source?.close()
  connected.value = false
})
</script>

<template>
  <section class="page space-y-8">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Job details
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Execution log
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Follow along as the workflow updates the local stack and routing.
        </p>
      </div>
      <UiButton :as="RouterLink" to="/jobs" variant="ghost" size="sm">
        Back to jobs
      </UiButton>
    </div>

    <UiState v-if="error" tone="error">
      {{ error }}
    </UiState>

    <UiState v-else-if="!job" loading>
      Loading job details...
    </UiState>

    <div v-else class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
      <UiPanel class="space-y-4 p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Job</p>
            <h2 class="mt-1 text-xl font-semibold text-[color:var(--text)]">
              {{ job.type }}
            </h2>
          </div>
          <UiBadge :tone="jobStatusTone(job.status)">
            {{ jobStatusLabel(job.status) }}
          </UiBadge>
        </div>

        <div class="grid gap-2 text-xs text-[color:var(--muted)]">
          <div class="flex items-center justify-between">
            <span>Started</span>
            <span class="font-semibold text-[color:var(--text)]">
              {{ job.startedAt ? new Date(job.startedAt).toLocaleString() : '--' }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span>Finished</span>
            <span class="font-semibold text-[color:var(--text)]">
              {{ job.finishedAt ? new Date(job.finishedAt).toLocaleString() : '--' }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span>Streaming</span>
            <span class="font-semibold text-[color:var(--text)]">
              {{ connected ? 'Connected' : streaming ? 'Starting...' : 'Stopped' }}
            </span>
          </div>
        </div>

        <UiState v-if="job.error" tone="error">
          {{ job.error }}
        </UiState>
      </UiPanel>

      <UiPanel class="p-5">
        <div class="flex items-center justify-between">
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Logs</p>
          <span class="text-xs text-[color:var(--muted-2)]">{{ logLines.length }} lines</span>
        </div>
        <pre
          class="mt-4 max-h-[420px] overflow-auto rounded-2xl border border-[color:var(--border)] bg-[color:var(--surface-inset)] p-4 text-xs text-[color:var(--text)]"
        ><code>{{ logLines.join('\n') || 'Waiting for log output...' }}</code></pre>
      </UiPanel>
    </div>
  </section>
</template>
