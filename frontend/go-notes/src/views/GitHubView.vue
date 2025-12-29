<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiOnboardingOverlay from '@/components/ui/UiOnboardingOverlay.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiState from '@/components/ui/UiState.vue'
import { settingsApi } from '@/services/settings'
import { apiErrorMessage } from '@/services/api'
import type { Settings } from '@/types/settings'
import type { OnboardingStep } from '@/types/onboarding'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const settings = ref<Settings | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const onboardingKey = 'warp-panel-onboarding-github'
const onboardingOpen = ref(false)
const onboardingStep = ref(0)

const onboardingSteps: OnboardingStep[] = [
  {
    id: 'token',
    title: 'Confirm GitHub token',
    description: 'Add a token so template creation and catalog sync are available.',
    target: "[data-onboard='github-token']",
    links: [
      {
        label: 'GitHub tokens',
        href: 'https://github.com/settings/tokens',
      },
    ],
  },
  {
    id: 'templates',
    title: 'Check template availability',
    description: 'Review the template sources and allowlist status for deploy readiness.',
    target: "[data-onboard='github-templates']",
  },
  {
    id: 'host-settings',
    title: 'Open host settings',
    description: 'Jump to Host Settings to update tokens or allowlist rules.',
    target: "[data-onboard='github-actions']",
  },
]

const hasToken = computed(() => Boolean(settings.value?.githubToken))

const tokenStatus = computed(() => {
  if (loading.value && !settings.value) return 'Checking'
  if (!settings.value) return 'Unknown'
  return hasToken.value ? 'Configured' : 'Missing'
})

const tokenTone = computed<BadgeTone>(() => {
  if (tokenStatus.value === 'Configured') return 'ok'
  if (tokenStatus.value === 'Missing') return 'warn'
  return 'neutral'
})

const templateStatus = computed(() => {
  if (loading.value && !settings.value) return 'Checking'
  if (!hasToken.value) return 'Needs token'
  return 'Awaiting sync'
})

const templateTone = computed<BadgeTone>(() => {
  if (templateStatus.value === 'Awaiting sync') return 'neutral'
  if (templateStatus.value === 'Needs token') return 'warn'
  return 'neutral'
})

const loadSettings = async () => {
  loading.value = true
  error.value = null
  try {
    const { data } = await settingsApi.get()
    settings.value = data.settings
  } catch (err) {
    error.value = apiErrorMessage(err)
  } finally {
    loading.value = false
  }
}

const startOnboarding = () => {
  onboardingStep.value = 0
  onboardingOpen.value = true
}

const markOnboardingComplete = () => {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(onboardingKey, 'done')
  }
}

onMounted(() => {
  loadSettings()
  if (typeof window !== 'undefined') {
    const seen = window.localStorage.getItem(onboardingKey)
    if (seen !== 'done') {
      onboardingOpen.value = true
    }
  }
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
      <div class="flex flex-wrap gap-3" data-onboard="github-actions">
        <UiButton variant="ghost" size="sm" @click="startOnboarding">
          View guide
        </UiButton>
        <UiButton variant="ghost" size="sm" :disabled="loading" @click="loadSettings">
          <span class="flex items-center gap-2">
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

    <div class="grid gap-6 lg:grid-cols-[1.1fr,0.9fr]">
      <UiPanel as="article" class="space-y-5 p-6" data-onboard="github-token">
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Token
            </p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              GitHub token status
            </h2>
          </div>
          <UiBadge :tone="tokenTone">
            {{ tokenStatus }}
          </UiBadge>
        </div>

        <p class="text-sm text-[color:var(--muted)]">
          The panel uses the token to create repos from templates and sync the
          available stack catalog.
        </p>

        <UiPanel v-if="loading && !settings" variant="soft" class="space-y-3 p-4">
          <UiSkeleton class="h-3 w-32" />
          <UiSkeleton class="h-3 w-full" />
          <UiSkeleton class="h-3 w-2/3" />
        </UiPanel>

        <div v-else class="space-y-3 text-xs text-[color:var(--muted)]">
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Repo creation</span>
            <span class="text-[color:var(--text)]">
              {{ hasToken ? 'Enabled' : 'Needs token' }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Template sync</span>
            <span class="text-[color:var(--text)]">
              {{ hasToken ? 'Ready' : 'Waiting' }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Allowlist checks</span>
            <span class="text-[color:var(--text)]">
              Pending API
            </span>
          </UiListRow>
        </div>
      </UiPanel>

      <UiPanel as="article" variant="raise" class="space-y-5 p-6" data-onboard="github-templates">
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
          Template repositories and allowlist visibility will appear here once
          the GitHub catalog sync is live.
        </p>

        <div v-if="loading && !settings" class="space-y-3">
          <UiSkeleton variant="block" class="h-16" />
          <UiSkeleton variant="block" class="h-16" />
        </div>

        <div v-else class="grid gap-3">
          <UiListRow class="space-y-2 text-xs text-[color:var(--muted)]">
            <p class="text-[11px] uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Template sources
            </p>
            <p>
              Waiting on the template inventory feed from the panel API.
            </p>
          </UiListRow>

          <UiListRow class="space-y-2 text-xs text-[color:var(--muted)]">
            <p class="text-[11px] uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Allowlist status
            </p>
            <p>
              No allowlist records loaded yet.
            </p>
          </UiListRow>
        </div>
      </UiPanel>
    </div>

    <UiOnboardingOverlay
      v-model="onboardingOpen"
      v-model:stepIndex="onboardingStep"
      :steps="onboardingSteps"
      @finish="markOnboardingComplete"
      @skip="markOnboardingComplete"
    />
  </section>
</template>
