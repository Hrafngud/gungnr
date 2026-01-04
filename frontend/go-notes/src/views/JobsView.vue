<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import { jobsApi } from '@/services/jobs'
import { apiErrorMessage } from '@/services/api'
import { useJobsStore } from '@/stores/jobs'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toasts'
import { usePageLoadingStore } from '@/stores/pageLoading'
import { isPendingJob, jobStatusLabel, jobStatusTone } from '@/utils/jobStatus'
import type { Job } from '@/types/jobs'

const jobsStore = useJobsStore()
const auth = useAuthStore()
const toastStore = useToastStore()
const pageLoading = usePageLoadingStore()
const stopping = ref<Record<number, boolean>>({})
const retrying = ref<Record<number, boolean>>({})

onMounted(async () => {
  pageLoading.start('Loading job timeline...')
  if (!jobsStore.initialized) {
    await jobsStore.fetchJobs()
  }
  pageLoading.stop()
})

const stopJob = async (job: Job) => {
  if (!isPendingJob(job.status)) return
  if (typeof window !== 'undefined') {
    const confirmed = window.confirm('Mark this job as failed?')
    if (!confirmed) return
  }
  stopping.value[job.id] = true
  try {
    await jobsApi.stop(job.id, { error: 'manually stopped' })
    toastStore.warn('Job marked failed.', 'Job stopped')
    await jobsStore.fetchJobs()
  } catch (err) {
    const message = apiErrorMessage(err)
    toastStore.error(message, 'Stop failed')
  } finally {
    stopping.value[job.id] = false
  }
}

const retryJob = async (job: Job) => {
  if (job.status !== 'failed') return
  retrying.value[job.id] = true
  try {
    await jobsApi.retry(job.id)
    toastStore.success('Job retry queued.', 'Job retried')
    await jobsStore.fetchJobs()
  } catch (err) {
    const message = apiErrorMessage(err)
    toastStore.error(message, 'Retry failed')
  } finally {
    retrying.value[job.id] = false
  }
}

</script>

<template>
  <section class="page space-y-8">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Jobs
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Automation timeline
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Deployment tasks will surface here with status and logs.
        </p>
      </div>
      <UiButton
        variant="ghost"
        size="sm"
        :disabled="jobsStore.loading"
        @click="jobsStore.fetchJobs"
      >
        <span class="flex items-center gap-2">
          <NavIcon name="refresh" class="h-3.5 w-3.5" />
          <UiInlineSpinner v-if="jobsStore.loading" />
          Refresh
        </span>
      </UiButton>
    </div>

    <UiPanel
      variant="soft"
      class="flex flex-wrap items-center justify-between gap-3 p-4 text-xs text-[color:var(--muted)]"
    >
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Day-to-day guidance
        </p>
        <p class="mt-1 text-sm text-[color:var(--muted)]">
          Run deploys or tunnel changes, then monitor status and open logs to confirm each step.
        </p>
      </div>
      <UiButton :as="RouterLink" to="/" variant="ghost" size="sm">
        Open quick deploy
      </UiButton>
    </UiPanel>

    <hr />

    <UiState v-if="jobsStore.error" tone="error">
      {{ jobsStore.error }}
    </UiState>

    <UiState v-else-if="jobsStore.loading" loading>
      Loading job history from the panel API...
    </UiState>

    <UiPanel
      v-else-if="jobsStore.jobs.length === 0"
      variant="raise"
      class="grid gap-6 p-6 lg:grid-cols-[1.05fr,0.95fr]"
    >
      <div>
        <h2 class="text-xl font-semibold text-[color:var(--text)]">No jobs yet</h2>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Once you run a template build, deploy, or tunnel update, the job status
          and logs will appear here.
        </p>
        <div class="mt-4 flex flex-wrap gap-3 text-xs text-[color:var(--muted)]">
          <UiButton as="span" variant="chip" size="chip">
            Template builds
          </UiButton>
          <UiButton as="span" variant="chip" size="chip">
            DNS updates
          </UiButton>
          <UiButton as="span" variant="chip" size="chip">
            Tunnel restarts
          </UiButton>
        </div>
      </div>
      <UiPanel variant="soft" class="p-4 text-sm text-[color:var(--muted)]">
        <p class="font-semibold text-[color:var(--text)]">Ready when you are</p>
        <p class="mt-2">
          Kick off a deployment flow to populate the timeline.
        </p>
        <UiButton
          v-if="!auth.user"
          :as="RouterLink"
          to="/login"
          variant="primary"
          size="md"
          class="mt-4 w-full justify-center"
        >
          <span class="flex items-center gap-2">
            <NavIcon name="login" class="h-4 w-4" />
            Sign in to continue
          </span>
        </UiButton>
      </UiPanel>
    </UiPanel>

    <div v-else class="space-y-4">
      <UiListRow
        v-for="job in jobsStore.jobs"
        :key="job.id"
        as="article"
        class="space-y-4"
      >
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Job</p>
            <h2 class="mt-1 text-lg font-semibold text-[color:var(--text)]">
              {{ job.type }}
            </h2>
          </div>
          <UiBadge :tone="jobStatusTone(job.status)">
            {{ jobStatusLabel(job.status) }}
          </UiBadge>
        </div>

        <div class="mt-4 grid gap-2 text-xs text-[color:var(--muted)] sm:grid-cols-2">
          <div class="flex items-center justify-between">
            <span>Started</span>
            <span class="font-semibold text-[color:var(--text)]">
              {{ job.startedAt ? new Date(job.startedAt).toLocaleString() : 'n/a' }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span>Finished</span>
            <span class="font-semibold text-[color:var(--text)]">
              {{ job.finishedAt ? new Date(job.finishedAt).toLocaleString() : 'n/a' }}
            </span>
          </div>
        </div>

        <UiState v-if="job.error" tone="error">
          {{ job.error }}
        </UiState>

        <div class="mt-4 flex flex-wrap items-center justify-between gap-3 text-xs text-[color:var(--muted)]">
          <span>Created {{ new Date(job.createdAt).toLocaleString() }}</span>
          <div class="flex flex-wrap items-center gap-2">
            <UiButton
              v-if="isPendingJob(job.status)"
              variant="ghost"
              size="sm"
              :disabled="stopping[job.id]"
              @click="stopJob(job)"
            >
              <span class="flex items-center gap-2">
                <UiInlineSpinner v-if="stopping[job.id]" />
                Mark failed
              </span>
            </UiButton>
            <UiButton
              v-if="job.status === 'failed'"
              variant="ghost"
              size="sm"
              :disabled="retrying[job.id]"
              @click="retryJob(job)"
            >
              <span class="flex items-center gap-2">
                <UiInlineSpinner v-if="retrying[job.id]" />
                Retry
              </span>
            </UiButton>
            <UiButton :as="RouterLink" :to="`/jobs/${job.id}`" variant="ghost" size="sm">
              View log
            </UiButton>
          </div>
        </div>
      </UiListRow>
    </div>
  </section>
</template>
