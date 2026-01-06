<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import { githubApi } from '@/services/github'
import { usersApi } from '@/services/users'
import { apiErrorMessage } from '@/services/api'
import { usePageLoadingStore } from '@/stores/pageLoading'
import { useAuthStore } from '@/stores/auth'
import type { GitHubCatalog } from '@/types/github'
import type { UserSummary } from '@/types/users'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const catalog = ref<GitHubCatalog | null>(null)
const allowlistUsers = ref<UserSummary[]>([])
const loading = ref(false)
const allowlistLoading = ref(false)
const error = ref<string | null>(null)
const pageLoading = usePageLoadingStore()
const authStore = useAuthStore()

const templateConfigured = computed(() => Boolean(catalog.value?.template.configured))
const isAdmin = computed(() => authStore.isAdmin)
const appConfigured = computed(() => Boolean(catalog.value?.app?.configured))

const installationTokenStatus = computed(() => {
  if (loading.value && !catalog.value) return 'Checking'
  if (!catalog.value) return 'Unknown'
  return appConfigured.value ? 'Configured' : 'Missing'
})

const installationTokenTone = computed<BadgeTone>(() => {
  if (installationTokenStatus.value === 'Configured') return 'ok'
  if (installationTokenStatus.value === 'Missing') return 'warn'
  return 'neutral'
})

const appStatus = computed(() => {
  if (loading.value && !catalog.value) return 'Checking'
  if (!catalog.value) return 'Unknown'
  return appConfigured.value ? 'Configured' : 'Missing'
})

const appTone = computed<BadgeTone>(() => {
  if (appStatus.value === 'Configured') return 'ok'
  if (appStatus.value === 'Missing') return 'warn'
  return 'neutral'
})

const templateStatus = computed(() => {
  if (loading.value && !catalog.value) return 'Checking'
  if (!catalog.value) return 'Unknown'
  if (!appConfigured.value) return 'Needs app setup'
  if (!templateConfigured.value) return 'Not configured'
  return 'Ready'
})

const templateTone = computed<BadgeTone>(() => {
  if (templateStatus.value === 'Ready') return 'ok'
  if (templateStatus.value === 'Needs app setup' || templateStatus.value === 'Not configured') {
    return 'warn'
  }
  return 'neutral'
})

const templateSyncLabel = computed(() => {
  if (!appConfigured.value) return 'Waiting'
  if (!templateConfigured.value) return 'Needs template'
  return 'Ready'
})

const allowlistUsersLabel = computed(() => {
  if (allowlistLoading.value && allowlistUsers.value.length === 0) return 'Checking'
  if (allowlistUsers.value.length === 0) return 'None'
  const names = allowlistUsers.value.map((user) => `@${user.login}`)
  const limit = 3
  const head = names.slice(0, limit).join(', ')
  if (names.length > limit) {
    return `${head} +${names.length - limit} more`
  }
  return head
})

const appIdStatus = computed(() => {
  if (loading.value && !catalog.value) return 'Checking'
  if (!catalog.value) return 'Unknown'
  return catalog.value.app.appIdConfigured ? 'Configured' : 'Missing'
})

const appInstallationStatus = computed(() => {
  if (loading.value && !catalog.value) return 'Checking'
  if (!catalog.value) return 'Unknown'
  return catalog.value.app.installationIdConfigured ? 'Configured' : 'Missing'
})

const appKeyStatus = computed(() => {
  if (loading.value && !catalog.value) return 'Checking'
  if (!catalog.value) return 'Unknown'
  return catalog.value.app.privateKeyConfigured ? 'Configured' : 'Missing'
})

const templateSource = computed(() => {
  if (!catalog.value?.template.configured) return 'Not configured'
  const owner = catalog.value.template.owner
  const repo = catalog.value.template.repo
  if (!owner || !repo) return 'Not configured'
  return `${owner}/${repo}`
})

const templateAccess = computed(() => catalog.value?.templateAccess)

const installationOwnerLabel = computed(() => {
  const owner = templateAccess.value?.installationOwner?.trim()
  if (!owner) return '--'
  const ownerType = templateAccess.value?.installationOwnerType?.trim()
  return ownerType ? `${owner} (${ownerType})` : owner
})

const templateTargetOwner = computed(() => {
  if (!catalog.value?.template.configured) return '--'
  return catalog.value.template.targetOwner || '--'
})

const templateVisibility = computed(() => {
  if (!catalog.value?.template.configured) return '--'
  return catalog.value.template.private ? 'Private' : 'Public'
})

const templateAccessStatus = computed(() => {
  if (loading.value && !catalog.value) return 'Checking'
  if (!catalog.value) return 'Unknown'
  if (!templateConfigured.value) return 'Not configured'
  const access = templateAccess.value?.repoAccess
  if (!access?.checked) {
    if (access?.error) return 'Error'
    return 'Not checked'
  }
  return access.available ? 'Accessible' : 'Blocked'
})

const templateAccessTone = computed<BadgeTone>(() => {
  if (templateAccessStatus.value === 'Accessible') return 'ok'
  if (templateAccessStatus.value === 'Blocked' || templateAccessStatus.value === 'Error') return 'warn'
  return 'neutral'
})

const templateAccessError = computed(() => templateAccess.value?.repoAccess?.error || '')
const templateAccessRequestId = computed(() => templateAccess.value?.repoAccess?.requestId || '')

const loadCatalog = async () => {
  const { data } = await githubApi.catalog()
  catalog.value = data.catalog
}

const loadAllowlist = async () => {
  const { data } = await usersApi.list()
  allowlistUsers.value = data.users ?? []
}

const loadGitHub = async () => {
  loading.value = true
  allowlistLoading.value = true
  error.value = null
  const [catalogResult, allowlistResult] = await Promise.allSettled([
    loadCatalog(),
    loadAllowlist(),
  ])

  if (catalogResult.status === 'rejected') {
    error.value = apiErrorMessage(catalogResult.reason)
  }

  if (allowlistResult.status === 'rejected') {
    error.value = error.value ?? apiErrorMessage(allowlistResult.reason)
  }

  loading.value = false
  allowlistLoading.value = false
}

onMounted(async () => {
  pageLoading.start('Loading GitHub catalog...')
  await loadGitHub()
  pageLoading.stop()
})
</script>

<template>
  <section class="page space-y-10">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          GitHub
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Template access
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Review token status and template availability for deploy workflows.
        </p>
      </div>
      <div class="flex flex-wrap gap-3">
        <UiButton variant="ghost" size="sm" :disabled="loading" @click="loadGitHub">
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="loading" />
            Refresh status
          </span>
        </UiButton>
        <UiButton :as="RouterLink" to="/host-settings" variant="primary" size="sm">
          Open host settings
        </UiButton>
      </div>
    </div>

    <UiState v-if="error" tone="error">
      {{ error }}
    </UiState>

    <UiState v-if="!isAdmin" tone="warn">
      Read-only access: admin permissions are required to update GitHub settings.
    </UiState>

    <hr />

    <div class="grid gap-6 lg:grid-cols-3">
      <UiPanel as="article" class="space-y-5 p-6">
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              App token
            </p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              Installation token status
            </h2>
          </div>
          <UiBadge :tone="installationTokenTone">
            {{ installationTokenStatus }}
          </UiBadge>
        </div>

        <p class="text-sm text-[color:var(--muted)]">
          Installation tokens are minted from the GitHub App to create repos
          from templates and sync the catalog.
        </p>

        <UiPanel v-if="loading && !catalog" variant="soft" class="space-y-3 p-4">
          <UiSkeleton class="h-3 w-32" />
          <UiSkeleton class="h-3 w-full" />
          <UiSkeleton class="h-3 w-2/3" />
        </UiPanel>

        <div v-else class="space-y-3 text-xs text-[color:var(--muted)]">
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Repo creation</span>
            <span class="text-[color:var(--text)]">
              {{ appConfigured ? 'Enabled' : 'Needs app setup' }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Template sync</span>
            <span class="text-[color:var(--text)]">
              {{ templateSyncLabel }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Allowlisted users</span>
            <span class="text-[color:var(--text)]">
              Users ({{ allowlistUsers.length }})
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Usernames</span>
            <span class="text-[color:var(--text)]">
              {{ allowlistUsersLabel }}
            </span>
          </UiListRow>
        </div>
      </UiPanel>

      <UiPanel as="article" variant="soft" class="space-y-5 p-6">
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              GitHub App
            </p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              App credential status
            </h2>
          </div>
          <UiBadge :tone="appTone">
            {{ appStatus }}
          </UiBadge>
        </div>

        <p class="text-sm text-[color:var(--muted)]">
          App credentials unlock installation tokens for template generation
          when a PAT is not set.
        </p>

        <UiPanel v-if="loading && !catalog" variant="soft" class="space-y-3 p-4">
          <UiSkeleton class="h-3 w-32" />
          <UiSkeleton class="h-3 w-full" />
          <UiSkeleton class="h-3 w-2/3" />
        </UiPanel>

        <div v-else class="space-y-3 text-xs text-[color:var(--muted)]">
          <UiListRow class="flex items-center justify-between gap-2">
            <span>App ID</span>
            <span class="text-[color:var(--text)]">
              {{ appIdStatus }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Installation ID</span>
            <span class="text-[color:var(--text)]">
              {{ appInstallationStatus }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Private key</span>
            <span class="text-[color:var(--text)]">
              {{ appKeyStatus }}
            </span>
          </UiListRow>
        </div>
      </UiPanel>

      <UiPanel as="article" variant="raise" class="space-y-5 p-6">
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Templates
            </p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              Template availability
            </h2>
          </div>
          <UiBadge :tone="templateTone">
            {{ templateStatus }}
          </UiBadge>
        </div>

        <p class="text-sm text-[color:var(--muted)]">
          Template repositories and destination ownership are pulled from the
          panel configuration.
        </p>

        <div v-if="loading && !catalog" class="space-y-3">
          <UiSkeleton variant="block" class="h-16" />
          <UiSkeleton variant="block" class="h-16" />
        </div>

        <div v-else class="space-y-3 text-xs text-[color:var(--muted)]">
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Template repo</span>
            <span class="text-[color:var(--text)]">
              {{ templateSource }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Installation owner</span>
            <span class="text-[color:var(--text)]">
              {{ installationOwnerLabel }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Template access</span>
            <UiBadge :tone="templateAccessTone">
              {{ templateAccessStatus }}
            </UiBadge>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Target owner</span>
            <span class="text-[color:var(--text)]">
              {{ templateTargetOwner }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Visibility</span>
            <span class="text-[color:var(--text)]">
              {{ templateVisibility }}
            </span>
          </UiListRow>
          <UiPanel
            v-if="templateAccessError"
            variant="soft"
            class="space-y-2 border border-[color:var(--border)] p-3 text-xs text-[color:var(--muted)]"
          >
            <div class="text-[color:var(--text)]">Access details</div>
            <div>{{ templateAccessError }}</div>
            <div v-if="templateAccessRequestId">
              Request ID: <span class="text-[color:var(--text)]">{{ templateAccessRequestId }}</span>
            </div>
          </UiPanel>
        </div>
      </UiPanel>
    </div>

  </section>
</template>
