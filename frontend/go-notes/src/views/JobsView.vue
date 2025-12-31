<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import { useJobsStore } from '@/stores/jobs'
import { useAuthStore } from '@/stores/auth'
import { jobStatusLabel, jobStatusTone } from '@/utils/jobStatus'

const jobsStore = useJobsStore()
const auth = useAuthStore()

onMounted(() => {
  if (!jobsStore.initialized) {
    jobsStore.fetchJobs()
  }
})

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
          Sign in to continue
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
          <UiButton :as="RouterLink" :to="`/jobs/${job.id}`" variant="ghost" size="sm">
            View log
          </UiButton>
        </div>
      </UiListRow>
    </div>
  </section>
</template>
