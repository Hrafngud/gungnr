<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { settingsApi } from '@/services/settings'
import { hostApi } from '@/services/host'
import { projectsApi } from '@/services/projects'
import { apiErrorMessage } from '@/services/api'
import type { CloudflaredPreview, Settings } from '@/types/settings'
import type { DockerContainer } from '@/types/host'

const settingsForm = reactive<Settings>({
  baseDomain: '',
  githubToken: '',
  cloudflareToken: '',
  cloudflaredConfigPath: '',
})

const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)
const success = ref<string | null>(null)

const preview = ref<CloudflaredPreview | null>(null)
const previewLoading = ref(false)
const previewError = ref<string | null>(null)

const containers = ref<DockerContainer[]>([])
const containersLoading = ref(false)
const containersError = ref<string | null>(null)

type ForwardState = {
  subdomain: string
  port: string
  loading: boolean
  error: string | null
  jobId: number | null
}

const forwardTargets = reactive<Record<string, ForwardState>>({})

const hasPreview = computed(() => Boolean(preview.value?.contents))

const statusTone = (status: string) => {
  if (status.toLowerCase().startsWith('up')) return 'bg-emerald-100 text-emerald-700'
  if (status.toLowerCase().startsWith('exited')) return 'bg-rose-100 text-rose-700'
  return 'bg-neutral-100 text-neutral-600'
}

const hostPortsFor = (container: DockerContainer) => {
  const ports = container.portBindings
    .filter((binding) => binding.published && binding.hostPort)
    .map((binding) => binding.hostPort)
  return Array.from(new Set(ports))
}

const ensureForwardState = (container: DockerContainer) => {
  if (!forwardTargets[container.id]) {
    const ports = hostPortsFor(container)
    const firstPort = ports[0]
    forwardTargets[container.id] = {
      subdomain: '',
      port: typeof firstPort === 'number' ? firstPort.toString() : '',
      loading: false,
      error: null,
      jobId: null,
    }
  }
}

const forwardStateFor = (container: DockerContainer): ForwardState => {
  ensureForwardState(container)
  return forwardTargets[container.id] as ForwardState
}

const loadSettings = async () => {
  loading.value = true
  error.value = null
  try {
    const { data } = await settingsApi.get()
    Object.assign(settingsForm, data.settings)
  } catch (err) {
    error.value = apiErrorMessage(err)
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  if (saving.value) return
  saving.value = true
  error.value = null
  success.value = null
  try {
    const { data } = await settingsApi.update({ ...settingsForm })
    Object.assign(settingsForm, data.settings)
    success.value = 'Settings saved.'
    await loadPreview()
  } catch (err) {
    error.value = apiErrorMessage(err)
  } finally {
    saving.value = false
  }
}

const loadPreview = async () => {
  previewLoading.value = true
  previewError.value = null
  try {
    const { data } = await settingsApi.preview()
    preview.value = data.preview
  } catch (err) {
    previewError.value = apiErrorMessage(err)
    preview.value = null
  } finally {
    previewLoading.value = false
  }
}

const loadContainers = async () => {
  containersLoading.value = true
  containersError.value = null
  try {
    const { data } = await hostApi.listDocker()
    containers.value = data.containers
    containers.value.forEach((container) => ensureForwardState(container))
  } catch (err) {
    containersError.value = apiErrorMessage(err)
  } finally {
    containersLoading.value = false
  }
}

const queueForward = async (container: DockerContainer) => {
  const state = forwardStateFor(container)
  state.error = null
  state.jobId = null

  const port = Number(state.port)
  if (!state.subdomain.trim()) {
    state.error = 'Subdomain is required.'
    return
  }
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    state.error = 'Select a valid host port.'
    return
  }

  state.loading = true
  try {
    const { data } = await projectsApi.quickService({
      subdomain: state.subdomain,
      port,
    })
    state.jobId = data.job.id
  } catch (err) {
    state.error = apiErrorMessage(err)
  } finally {
    state.loading = false
  }
}

onMounted(async () => {
  await Promise.all([loadSettings(), loadPreview(), loadContainers()])
})
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">Settings</p>
        <h1 class="mt-2 text-3xl font-semibold text-neutral-900">
          Runtime configuration
        </h1>
        <p class="mt-2 text-sm text-neutral-600">
          These values override the backend defaults and power the tunnel, DNS,
          and template workflows.
        </p>
      </div>
      <button
        type="button"
        class="rounded-full border border-black/10 bg-white/80 px-4 py-2 text-xs font-semibold uppercase tracking-[0.2em] text-neutral-700 transition hover:-translate-y-0.5"
        @click="loadContainers"
      >
        Refresh host data
      </button>
    </div>

    <div
      v-if="error"
      class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700"
    >
      {{ error }}
    </div>

    <div
      v-if="success"
      class="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
    >
      {{ success }}
    </div>

    <div class="grid gap-6 lg:grid-cols-[1.1fr,0.9fr]">
      <form
        class="space-y-5 rounded-[28px] border border-black/10 bg-white/80 p-6 shadow-[0_25px_70px_-55px_rgba(0,0,0,0.5)]"
        @submit.prevent="saveSettings"
      >
        <div class="flex items-center justify-between gap-4">
          <h2 class="text-lg font-semibold text-neutral-900">Panel settings</h2>
          <span
            class="rounded-full border border-black/10 bg-white/80 px-3 py-1 text-[11px] uppercase tracking-[0.2em] text-neutral-500"
          >
            Overrides
          </span>
        </div>

        <div class="grid gap-4 text-sm text-neutral-700">
          <label class="grid gap-2">
            <span class="text-xs uppercase tracking-[0.3em] text-neutral-500">
              Base domain
            </span>
            <input
              v-model="settingsForm.baseDomain"
              type="text"
              placeholder="example.com"
              class="rounded-2xl border border-black/10 bg-white/90 px-4 py-2 text-sm"
              :disabled="loading"
            />
          </label>

          <label class="grid gap-2">
            <span class="text-xs uppercase tracking-[0.3em] text-neutral-500">
              GitHub token
            </span>
            <input
              v-model="settingsForm.githubToken"
              type="password"
              placeholder="ghp_••••••"
              class="rounded-2xl border border-black/10 bg-white/90 px-4 py-2 text-sm"
              :disabled="loading"
            />
          </label>

          <label class="grid gap-2">
            <span class="text-xs uppercase tracking-[0.3em] text-neutral-500">
              Cloudflare API token
            </span>
            <input
              v-model="settingsForm.cloudflareToken"
              type="password"
              placeholder="cf_••••••"
              class="rounded-2xl border border-black/10 bg-white/90 px-4 py-2 text-sm"
              :disabled="loading"
            />
          </label>

          <label class="grid gap-2">
            <span class="text-xs uppercase tracking-[0.3em] text-neutral-500">
              Cloudflared config path
            </span>
            <input
              v-model="settingsForm.cloudflaredConfigPath"
              type="text"
              placeholder="~/.cloudflared/config.yml"
              class="rounded-2xl border border-black/10 bg-white/90 px-4 py-2 text-sm"
              :disabled="loading"
            />
          </label>
        </div>

        <div class="flex flex-wrap gap-3">
          <button
            type="submit"
            class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-[color:var(--accent-soft)] px-5 py-2 text-sm font-semibold text-[color:var(--accent-ink)] transition hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="saving || loading"
          >
            {{ saving ? 'Saving...' : 'Save settings' }}
          </button>
          <button
            type="button"
            class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-white/80 px-5 py-2 text-sm font-semibold text-neutral-700 transition hover:-translate-y-0.5"
            @click="loadSettings"
          >
            Reload
          </button>
        </div>
      </form>

      <div class="space-y-4 rounded-[28px] border border-black/10 bg-white/85 p-6">
        <div class="flex items-center justify-between gap-2">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
              Cloudflared config
            </p>
            <h3 class="mt-2 text-lg font-semibold text-neutral-900">
              Live ingress preview
            </h3>
          </div>
          <button
            type="button"
            class="rounded-full border border-black/10 bg-white/80 px-3 py-1 text-[11px] uppercase tracking-[0.2em] text-neutral-500 transition hover:-translate-y-0.5"
            @click="loadPreview"
          >
            Refresh
          </button>
        </div>

        <p class="text-sm text-neutral-600">
          Previewing {{ preview?.path || 'cloudflared config' }}.
        </p>

        <div
          v-if="previewLoading"
          class="rounded-2xl border border-dashed border-black/10 bg-white/70 p-4 text-xs text-neutral-500"
        >
          Loading config preview...
        </div>

        <div
          v-else-if="previewError"
          class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-700"
        >
          {{ previewError }}
        </div>

        <pre
          v-else-if="hasPreview"
          class="max-h-80 overflow-auto rounded-2xl border border-black/10 bg-neutral-900/90 p-4 text-xs text-emerald-200"
        ><code>{{ preview?.contents }}</code></pre>

        <div
          v-else
          class="rounded-2xl border border-black/10 bg-white/70 p-4 text-xs text-neutral-500"
        >
          Cloudflared config not loaded yet.
        </div>
      </div>
    </div>

    <div class="space-y-4">
      <div class="flex items-center justify-between gap-4">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">Host</p>
          <h2 class="mt-2 text-2xl font-semibold text-neutral-900">
            Running containers
          </h2>
        </div>
        <button
          type="button"
          class="rounded-full border border-black/10 bg-white/80 px-4 py-2 text-xs font-semibold uppercase tracking-[0.2em] text-neutral-700 transition hover:-translate-y-0.5"
          @click="loadContainers"
        >
          Refresh list
        </button>
      </div>

      <div
        v-if="containersError"
        class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700"
      >
        {{ containersError }}
      </div>

      <div
        v-if="containersLoading"
        class="rounded-[28px] border border-dashed border-black/10 bg-white/70 p-6 text-sm text-neutral-500"
      >
        Loading Docker containers...
      </div>

      <div
        v-else-if="containers.length === 0"
        class="rounded-[28px] border border-black/10 bg-white/80 p-6 text-sm text-neutral-600"
      >
        No running containers detected on the host.
      </div>

      <div v-else class="grid gap-4 lg:grid-cols-2">
        <article
          v-for="container in containers"
          :key="container.id"
          class="rounded-[24px] border border-black/10 bg-white/90 p-5 shadow-[0_25px_60px_-45px_rgba(0,0,0,0.45)]"
        >
          <div class="flex items-start justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
                {{ container.service || 'Container' }}
              </p>
              <h3 class="mt-2 text-lg font-semibold text-neutral-900">
                {{ container.name }}
              </h3>
              <p class="mt-1 text-xs text-neutral-500">
                {{ container.image }}
              </p>
            </div>
            <span
              class="rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em]"
              :class="statusTone(container.status)"
            >
              {{ container.status }}
            </span>
          </div>

          <div class="mt-4 space-y-2 text-xs text-neutral-600">
            <div class="flex items-center justify-between gap-2">
              <span>Ports</span>
              <span class="text-neutral-900">
                {{ container.ports || '—' }}
              </span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span>Project</span>
              <span class="text-neutral-900">
                {{ container.project || 'n/a' }}
              </span>
            </div>
          </div>

          <div class="mt-4 rounded-2xl border border-black/10 bg-white/80 p-4">
            <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
              Tunnel forward
            </p>
            <div class="mt-3 grid gap-3 text-xs text-neutral-600 sm:grid-cols-[1.2fr,0.8fr]">
              <input
                v-model="forwardStateFor(container).subdomain"
                type="text"
                placeholder="subdomain"
                class="rounded-2xl border border-black/10 bg-white/90 px-3 py-2 text-xs text-neutral-800"
              />
              <select
                v-model="forwardStateFor(container).port"
                class="rounded-2xl border border-black/10 bg-white/90 px-3 py-2 text-xs text-neutral-800"
              >
                <option value="">Select port</option>
                <option
                  v-for="port in hostPortsFor(container)"
                  :key="port"
                  :value="port"
                >
                  {{ port }}
                </option>
              </select>
            </div>

            <div class="mt-3 flex flex-wrap items-center gap-3">
              <button
                type="button"
                class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-[color:var(--accent-soft)] px-4 py-2 text-xs font-semibold text-[color:var(--accent-ink)] transition hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="forwardStateFor(container).loading"
                @click="queueForward(container)"
              >
                {{
                  forwardStateFor(container).loading
                    ? 'Forwarding...'
                    : 'Forward via tunnel'
                }}
              </button>

              <RouterLink
                v-if="forwardStateFor(container).jobId"
                :to="`/jobs/${forwardStateFor(container).jobId}`"
                class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-xs font-semibold text-neutral-700 transition hover:-translate-y-0.5"
              >
                View job
              </RouterLink>
            </div>

            <p
              v-if="forwardStateFor(container).error"
              class="mt-3 text-xs text-rose-600"
            >
              {{ forwardStateFor(container).error }}
            </p>
          </div>
        </article>
      </div>
    </div>
  </section>
</template>
