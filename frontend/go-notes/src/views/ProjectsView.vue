<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useProjectsStore } from '@/stores/projects'
import { useAuthStore } from '@/stores/auth'

const projectsStore = useProjectsStore()
const auth = useAuthStore()

onMounted(() => {
  if (!projectsStore.initialized) {
    projectsStore.fetchProjects()
  }
})

const statusTone = (status: string) => {
  if (status === 'running') return 'bg-emerald-100 text-emerald-700'
  if (status === 'stopped') return 'bg-amber-100 text-amber-700'
  return 'bg-neutral-100 text-neutral-600'
}

const formatPort = (value: number) => (value ? value.toString() : '—')
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
          Projects
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-neutral-900">
          Active stacks
        </h1>
        <p class="mt-2 text-sm text-neutral-600">
          Track local template deployments and their exposed ports.
        </p>
      </div>
      <div class="flex flex-wrap gap-3">
        <button
          type="button"
          class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm font-semibold text-neutral-700 transition hover:-translate-y-0.5"
          @click="projectsStore.fetchProjects"
        >
          Refresh
        </button>
        <div
          class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-[color:var(--accent-soft)] px-4 py-2 text-sm font-semibold text-[color:var(--accent-ink)]"
        >
          Create from template
        </div>
      </div>
    </div>

    <div
      v-if="projectsStore.error"
      class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700"
    >
      {{ projectsStore.error }}
    </div>

    <div
      v-if="projectsStore.loading"
      class="rounded-[28px] border border-dashed border-black/10 bg-white/70 p-6 text-sm text-neutral-500"
    >
      Loading projects from the panel API...
    </div>

    <div
      v-else-if="projectsStore.projects.length === 0"
      class="grid gap-6 rounded-[28px] border border-black/10 bg-white/80 p-6 lg:grid-cols-[1.1fr,0.9fr]"
    >
      <div>
        <h2 class="text-xl font-semibold text-neutral-900">No projects yet</h2>
        <p class="mt-2 text-sm text-neutral-600">
          Once you create or import a template stack, it will appear here with
          status, ports, and recent deployment details.
        </p>
        <div class="mt-4 flex flex-wrap gap-3 text-xs text-neutral-600">
          <span class="rounded-full border border-black/10 bg-white/70 px-3 py-1">
            GitHub template
          </span>
          <span class="rounded-full border border-black/10 bg-white/70 px-3 py-1">
            Docker compose
          </span>
          <span class="rounded-full border border-black/10 bg-white/70 px-3 py-1">
            Cloudflared ingress
          </span>
        </div>
      </div>
      <div class="rounded-2xl border border-black/10 bg-white/90 p-4 text-sm text-neutral-600">
        <p class="font-semibold text-neutral-800">Next step</p>
        <p class="mt-2">
          Wire the Create from template wizard to spin up a new stack and record
          it here.
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

    <div v-else class="grid gap-4 md:grid-cols-2">
      <article
        v-for="project in projectsStore.projects"
        :key="project.id"
        class="rounded-[24px] border border-black/10 bg-white/90 p-5 shadow-[0_25px_60px_-45px_rgba(0,0,0,0.5)]"
      >
        <div class="flex items-center justify-between gap-2">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">Stack</p>
            <h2 class="mt-1 text-lg font-semibold text-neutral-900">
              {{ project.name }}
            </h2>
          </div>
          <span
            class="rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em]"
            :class="statusTone(project.status)"
          >
            {{ project.status || 'unknown' }}
          </span>
        </div>

        <div class="mt-4 grid gap-3 text-xs text-neutral-600">
          <div class="flex items-center justify-between">
            <span>Proxy port</span>
            <span class="font-semibold text-neutral-900">
              {{ formatPort(project.proxyPort) }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span>Database port</span>
            <span class="font-semibold text-neutral-900">
              {{ formatPort(project.dbPort) }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span>Repo</span>
            <span class="truncate font-semibold text-neutral-900">
              {{ project.repoUrl || '—' }}
            </span>
          </div>
        </div>

        <div class="mt-4 text-xs text-neutral-500">
          Added {{ new Date(project.createdAt).toLocaleString() }}
        </div>
      </article>
    </div>
  </section>
</template>
