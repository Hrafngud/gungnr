<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { jobsApi } from '@/services/jobs'
import { apiErrorMessage, getApiBaseUrl } from '@/services/api'
import type { JobDetail } from '@/types/jobs'

const route = useRoute()
const job = ref<JobDetail | null>(null)
const logLines = ref<string[]>([])
const error = ref<string | null>(null)
const streaming = ref(false)
const connected = ref(false)
let source: EventSource | null = null

const jobId = Number(route.params.id)

const statusTone = (status: string) => {
  if (status === 'completed') return 'bg-emerald-100 text-emerald-700'
  if (status === 'running') return 'bg-sky-100 text-sky-700'
  if (status === 'failed') return 'bg-rose-100 text-rose-700'
  return 'bg-neutral-100 text-neutral-600'
}

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
  await fetchJob()
  if (job.value && job.value.status !== 'completed' && job.value.status !== 'failed') {
    const seed = logLines.value.join('\n')
    const offset = seed.length > 0 ? seed.length + 1 : 0
    startStream(offset)
  }
})

onBeforeUnmount(() => {
  source?.close()
  connected.value = false
})
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
          Job details
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-neutral-900">
          Execution log
        </h1>
        <p class="mt-2 text-sm text-neutral-600">
          Follow along as the workflow updates the local stack and tunnel.
        </p>
      </div>
      <RouterLink
        to="/jobs"
        class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm font-semibold text-neutral-700 transition hover:-translate-y-0.5"
      >
        Back to jobs
      </RouterLink>
    </div>

    <div
      v-if="error"
      class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700"
    >
      {{ error }}
    </div>

    <div v-else-if="!job" class="rounded-[28px] border border-dashed border-black/10 bg-white/70 p-6 text-sm text-neutral-500">
      Loading job details...
    </div>

    <div v-else class="grid gap-6 lg:grid-cols-[1fr,1fr]">
      <div class="space-y-4 rounded-[24px] border border-black/10 bg-white/90 p-5">
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">Job</p>
            <h2 class="mt-1 text-xl font-semibold text-neutral-900">
              {{ job.type }}
            </h2>
          </div>
          <span
            class="rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em]"
            :class="statusTone(job.status)"
          >
            {{ job.status || 'pending' }}
          </span>
        </div>

        <div class="grid gap-2 text-xs text-neutral-600">
          <div class="flex items-center justify-between">
            <span>Started</span>
            <span class="font-semibold text-neutral-900">
              {{ job.startedAt ? new Date(job.startedAt).toLocaleString() : '--' }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span>Finished</span>
            <span class="font-semibold text-neutral-900">
              {{ job.finishedAt ? new Date(job.finishedAt).toLocaleString() : '--' }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span>Streaming</span>
            <span class="font-semibold text-neutral-900">
              {{ connected ? 'Connected' : streaming ? 'Starting...' : 'Stopped' }}
            </span>
          </div>
        </div>

        <div
          v-if="job.error"
          class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-700"
        >
          {{ job.error }}
        </div>
      </div>

      <div class="rounded-[24px] border border-black/10 bg-white/90 p-5">
        <div class="flex items-center justify-between">
          <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">Logs</p>
          <span class="text-xs text-neutral-500">{{ logLines.length }} lines</span>
        </div>
        <pre
          class="mt-4 max-h-[420px] overflow-auto rounded-2xl border border-black/10 bg-neutral-950/90 p-4 text-xs text-neutral-100"
        ><code>{{ logLines.join('\n') || 'Waiting for log output...' }}</code></pre>
      </div>
    </div>
  </section>
</template>
