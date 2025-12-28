<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useProjectsStore } from '@/stores/projects'
import { useAuthStore } from '@/stores/auth'
import { projectsApi } from '@/services/projects'
import { apiErrorMessage } from '@/services/api'
import type { LocalProject } from '@/types/projects'

const projectsStore = useProjectsStore()
const auth = useAuthStore()
const router = useRouter()

const flow = ref<'template' | 'existing' | 'quick'>('template')
const localProjects = ref<LocalProject[]>([])
const localLoading = ref(false)
const localError = ref<string | null>(null)
const submitting = ref(false)
const submitError = ref<string | null>(null)
const submitSuccess = ref<string | null>(null)
const lastJobId = ref<number | null>(null)

const templateForm = reactive({
  name: '',
  subdomain: '',
  proxyPort: '',
  dbPort: '',
})

const existingForm = reactive({
  name: '',
  subdomain: '',
  port: '80',
})

const quickForm = reactive({
  subdomain: '',
  port: '',
})

onMounted(() => {
  if (!projectsStore.initialized) {
    projectsStore.fetchProjects()
  }
})

watch(flow, (value) => {
  submitError.value = null
  submitSuccess.value = null
  lastJobId.value = null
  if (value === 'existing' && localProjects.value.length === 0 && !localLoading.value) {
    loadLocalProjects()
  }
})

const statusTone = (status: string) => {
  if (status === 'running') return 'bg-emerald-100 text-emerald-700'
  if (status === 'stopped') return 'bg-amber-100 text-amber-700'
  return 'bg-neutral-100 text-neutral-600'
}

const formatPort = (value: number) => (value ? value.toString() : '—')

const canSubmit = computed(() => !submitting.value)

const parsePort = (value: string, required: boolean) => {
  const trimmed = value.trim()
  if (!trimmed) return required ? null : undefined
  const parsed = Number(trimmed)
  if (!Number.isInteger(parsed)) return null
  return parsed
}

const loadLocalProjects = async () => {
  localLoading.value = true
  localError.value = null
  try {
    const { data } = await projectsApi.listLocal()
    localProjects.value = data.projects
  } catch (err) {
    localError.value = apiErrorMessage(err)
  } finally {
    localLoading.value = false
  }
}

const handleSubmit = async () => {
  if (!canSubmit.value) return
  submitting.value = true
  submitError.value = null
  submitSuccess.value = null
  lastJobId.value = null

  try {
    if (flow.value === 'template') {
      if (!templateForm.name.trim()) {
        submitError.value = 'Project name is required.'
        submitting.value = false
        return
      }
      const proxyPort = parsePort(templateForm.proxyPort, false)
      const dbPort = parsePort(templateForm.dbPort, false)
      if (proxyPort === null || dbPort === null) {
        submitError.value = 'Ports must be numeric.'
        submitting.value = false
        return
      }
      const { data } = await projectsApi.createFromTemplate({
        name: templateForm.name,
        subdomain: templateForm.subdomain || undefined,
        proxyPort,
        dbPort,
      })
      lastJobId.value = data.job.id
    }

    if (flow.value === 'existing') {
      if (!existingForm.name.trim() || !existingForm.subdomain.trim()) {
        submitError.value = 'Project name and subdomain are required.'
        submitting.value = false
        return
      }
      const port = parsePort(existingForm.port, false)
      if (port === null) {
        submitError.value = 'Port must be numeric.'
        submitting.value = false
        return
      }
      const { data } = await projectsApi.deployExisting({
        name: existingForm.name,
        subdomain: existingForm.subdomain,
        port,
      })
      lastJobId.value = data.job.id
    }

    if (flow.value === 'quick') {
      if (!quickForm.subdomain.trim()) {
        submitError.value = 'Subdomain is required.'
        submitting.value = false
        return
      }
      const port = parsePort(quickForm.port, true)
      if (port === null) {
        submitError.value = 'Port must be numeric.'
        submitting.value = false
        return
      }
      const { data } = await projectsApi.quickService({
        subdomain: quickForm.subdomain,
        port,
      })
      lastJobId.value = data.job.id
    }

    submitSuccess.value = 'Job queued. Track the logs to follow progress.'
    projectsStore.fetchProjects()
  } catch (err) {
    submitError.value = apiErrorMessage(err)
  } finally {
    submitting.value = false
  }
}

const navigateToJob = () => {
  if (lastJobId.value) {
    router.push(`/jobs/${lastJobId.value}`)
  }
}
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
      </div>
    </div>

    <div class="grid gap-6 rounded-[28px] border border-black/10 bg-white/90 p-6 lg:grid-cols-[1.15fr,0.85fr]">
      <div class="space-y-6">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
            Deployment workflows
          </p>
          <h2 class="mt-2 text-2xl font-semibold text-neutral-900">
            Launch a new stack
          </h2>
          <p class="mt-2 text-sm text-neutral-600">
            Queue a job to create a template project, deploy an existing stack, or
            expose a quick local service.
          </p>
        </div>

        <div class="flex flex-wrap gap-2 text-xs font-semibold uppercase tracking-[0.2em] text-neutral-600">
          <button
            type="button"
            class="rounded-full px-4 py-2 transition"
            :class="flow === 'template' ? 'bg-[color:var(--accent-soft)] text-[color:var(--accent-ink)]' : 'bg-white/70'"
            @click="flow = 'template'"
          >
            Template
          </button>
          <button
            type="button"
            class="rounded-full px-4 py-2 transition"
            :class="flow === 'existing' ? 'bg-[color:var(--accent-soft)] text-[color:var(--accent-ink)]' : 'bg-white/70'"
            @click="flow = 'existing'"
          >
            Existing
          </button>
          <button
            type="button"
            class="rounded-full px-4 py-2 transition"
            :class="flow === 'quick' ? 'bg-[color:var(--accent-soft)] text-[color:var(--accent-ink)]' : 'bg-white/70'"
            @click="flow = 'quick'"
          >
            Quick service
          </button>
        </div>

        <form class="space-y-4" @submit.prevent="handleSubmit">
          <div v-if="flow === 'template'" class="space-y-4">
            <div>
              <label class="text-xs uppercase tracking-[0.3em] text-neutral-500">
                Project name
              </label>
              <input
                v-model="templateForm.name"
                type="text"
                class="mt-2 w-full rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm"
                placeholder="warp-ops"
              />
            </div>
            <div>
              <label class="text-xs uppercase tracking-[0.3em] text-neutral-500">
                Subdomain (optional)
              </label>
              <input
                v-model="templateForm.subdomain"
                type="text"
                class="mt-2 w-full rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm"
                placeholder="warp-ops"
              />
            </div>
            <div class="grid gap-4 sm:grid-cols-2">
              <div>
                <label class="text-xs uppercase tracking-[0.3em] text-neutral-500">
                  Proxy port
                </label>
                <input
                  v-model="templateForm.proxyPort"
                  type="text"
                  class="mt-2 w-full rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm"
                  placeholder="80"
                />
              </div>
              <div>
                <label class="text-xs uppercase tracking-[0.3em] text-neutral-500">
                  Database port
                </label>
                <input
                  v-model="templateForm.dbPort"
                  type="text"
                  class="mt-2 w-full rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm"
                  placeholder="5432"
                />
              </div>
            </div>
          </div>

          <div v-else-if="flow === 'existing'" class="space-y-4">
            <div>
              <label class="text-xs uppercase tracking-[0.3em] text-neutral-500">
                Project folder
              </label>
              <input
                v-model="existingForm.name"
                list="local-projects"
                type="text"
                class="mt-2 w-full rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm"
                placeholder="template-folder"
              />
              <datalist id="local-projects">
                <option v-for="project in localProjects" :key="project.name" :value="project.name" />
              </datalist>
              <p v-if="localLoading" class="mt-2 text-xs text-neutral-500">
                Loading local templates...
              </p>
              <p v-else-if="localError" class="mt-2 text-xs text-rose-600">
                {{ localError }}
              </p>
            </div>
            <div>
              <label class="text-xs uppercase tracking-[0.3em] text-neutral-500">
                Subdomain
              </label>
              <input
                v-model="existingForm.subdomain"
                type="text"
                class="mt-2 w-full rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm"
                placeholder="warp-ops"
              />
            </div>
            <div>
              <label class="text-xs uppercase tracking-[0.3em] text-neutral-500">
                Host port
              </label>
              <input
                v-model="existingForm.port"
                type="text"
                class="mt-2 w-full rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm"
                placeholder="80"
              />
            </div>
          </div>

          <div v-else class="space-y-4">
            <div>
              <label class="text-xs uppercase tracking-[0.3em] text-neutral-500">
                Subdomain
              </label>
              <input
                v-model="quickForm.subdomain"
                type="text"
                class="mt-2 w-full rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm"
                placeholder="preview"
              />
            </div>
            <div>
              <label class="text-xs uppercase tracking-[0.3em] text-neutral-500">
                Local port
              </label>
              <input
                v-model="quickForm.port"
                type="text"
                class="mt-2 w-full rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm"
                placeholder="5173"
              />
            </div>
          </div>

          <div
            v-if="submitError"
            class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-700"
          >
            {{ submitError }}
          </div>

          <div
            v-if="submitSuccess"
            class="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-xs text-emerald-700"
          >
            {{ submitSuccess }}
          </div>

          <div class="flex flex-wrap gap-3">
            <button
              type="submit"
              class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-[color:var(--accent-soft)] px-5 py-2 text-sm font-semibold text-[color:var(--accent-ink)] transition hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-60"
              :disabled="submitting"
            >
              {{ submitting ? 'Queueing...' : 'Queue job' }}
            </button>
            <button
              v-if="lastJobId"
              type="button"
              class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-white/80 px-5 py-2 text-sm font-semibold text-neutral-700 transition hover:-translate-y-0.5"
              @click="navigateToJob"
            >
              View job log
            </button>
          </div>
        </form>
      </div>

      <div class="rounded-2xl border border-black/10 bg-white/80 p-5 text-sm text-neutral-600">
        <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">Checklist</p>
        <h3 class="mt-2 text-lg font-semibold text-neutral-900">
          Before you deploy
        </h3>
        <ul class="mt-3 space-y-2 text-xs text-neutral-600">
          <li>Templates dir mounted into the API container.</li>
          <li>GitHub token and template repo set in the env.</li>
          <li>Cloudflared config path + tunnel name configured.</li>
          <li>DNS zone ready for new subdomains.</li>
        </ul>
        <RouterLink
          v-if="!auth.user"
          to="/login"
          class="mt-4 inline-flex w-full items-center justify-center rounded-2xl border border-black/10 bg-[color:var(--accent-soft)] px-4 py-2 text-sm font-semibold text-[color:var(--accent-ink)]"
        >
          Sign in to continue
        </RouterLink>
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
