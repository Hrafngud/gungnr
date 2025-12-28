<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useJobsStore } from '@/stores/jobs'
import { useAuthStore } from '@/stores/auth'

const jobsStore = useJobsStore()
const auth = useAuthStore()

onMounted(() => {
  if (!jobsStore.initialized) {
    jobsStore.fetchJobs()
  }
})

const statusTone = (status: string) => {
  if (status === 'completed') return 'bg-emerald-100 text-emerald-700'
  if (status === 'running') return 'bg-sky-100 text-sky-700'
  if (status === 'failed') return 'bg-rose-100 text-rose-700'
  return 'bg-neutral-100 text-neutral-600'
}
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
          Jobs
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-neutral-900">
          Automation timeline
        </h1>
        <p class="mt-2 text-sm text-neutral-600">
          Deployment tasks will surface here with status and logs.
        </p>
      </div>
      <button
        type="button"
        class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm font-semibold text-neutral-700 transition hover:-translate-y-0.5"
        @click="jobsStore.fetchJobs"
      >
        Refresh
      </button>
    </div>

    <div
      v-if="jobsStore.error"
      class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700"
    >
      {{ jobsStore.error }}
    </div>

    <div
      v-if="jobsStore.loading"
      class="rounded-[28px] border border-dashed border-black/10 bg-white/70 p-6 text-sm text-neutral-500"
    >
      Loading job history from the panel API...
    </div>

    <div
      v-else-if="jobsStore.jobs.length === 0"
      class="grid gap-6 rounded-[28px] border border-black/10 bg-white/80 p-6 lg:grid-cols-[1.05fr,0.95fr]"
    >
      <div>
        <h2 class="text-xl font-semibold text-neutral-900">No jobs yet</h2>
        <p class="mt-2 text-sm text-neutral-600">
          Once you run a template build, deploy, or tunnel update, the job status
          and logs will appear here.
        </p>
        <div class="mt-4 flex flex-wrap gap-3 text-xs text-neutral-600">
          <span class="rounded-full border border-black/10 bg-white/70 px-3 py-1">
            Template builds
          </span>
          <span class="rounded-full border border-black/10 bg-white/70 px-3 py-1">
            DNS updates
          </span>
          <span class="rounded-full border border-black/10 bg-white/70 px-3 py-1">
            Tunnel restarts
          </span>
        </div>
      </div>
      <div class="rounded-2xl border border-black/10 bg-white/90 p-4 text-sm text-neutral-600">
        <p class="font-semibold text-neutral-800">Ready when you are</p>
        <p class="mt-2">
          Kick off a deployment flow to populate the timeline.
        </p>
        <RouterLink
          v-if="!auth.user"
          to="/login"
          class="mt-4 inline-flex w-full items-center justify-center rounded-2xl border border-black/10 bg-[color:var(--accent-soft)] px-4 py-2 text-sm font-semibold text-[color:var(--accent-ink)]"
        >
          Sign in to continue
        </RouterLink>
      </div>
    </div>

    <div v-else class="space-y-4">
      <article
        v-for="job in jobsStore.jobs"
        :key="job.id"
        class="rounded-[24px] border border-black/10 bg-white/90 p-5 shadow-[0_20px_50px_-40px_rgba(0,0,0,0.55)]"
      >
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">Job</p>
            <h2 class="mt-1 text-lg font-semibold text-neutral-900">
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

        <div class="mt-4 grid gap-2 text-xs text-neutral-600 sm:grid-cols-2">
          <div class="flex items-center justify-between">
            <span>Started</span>
            <span class="font-semibold text-neutral-900">
              {{ job.startedAt ? new Date(job.startedAt).toLocaleString() : '—' }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span>Finished</span>
            <span class="font-semibold text-neutral-900">
              {{ job.finishedAt ? new Date(job.finishedAt).toLocaleString() : '—' }}
            </span>
          </div>
        </div>

        <div
          v-if="job.error"
          class="mt-4 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-700"
        >
          {{ job.error }}
        </div>

        <div class="mt-4 flex flex-wrap items-center justify-between gap-3 text-xs text-neutral-500">
          <span>Created {{ new Date(job.createdAt).toLocaleString() }}</span>
          <RouterLink
            :to="`/jobs/${job.id}`"
            class="rounded-full border border-black/10 bg-white/80 px-3 py-1 text-xs font-semibold text-neutral-700 transition hover:-translate-y-0.5"
          >
            View log
          </RouterLink>
        </div>
      </article>
    </div>
  </section>
</template>
