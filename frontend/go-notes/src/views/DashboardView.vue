<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import NavIcon from '@/components/NavIcon.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()

onMounted(() => {
  if (!auth.initialized) {
    auth.fetchUser()
  }
})

const greeting = computed(() => {
  if (auth.user) return `Welcome back, ${auth.user.login}.`
  if (auth.loading) return 'Checking your session...'
  return 'You are not signed in.'
})
</script>

<template>
  <section class="grid gap-8 lg:grid-cols-[1.2fr,0.8fr]">
    <div class="space-y-6 rounded-[28px] border border-black/10 bg-white/70 p-6 shadow-[0_25px_70px_-50px_rgba(0,0,0,0.65)]">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
          Control center
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-neutral-900">
          Deployment runway
        </h1>
        <p class="mt-3 max-w-2xl text-sm text-neutral-600">
          Track template creation, local service exposure, and tunnel activity in one place.
          This view confirms your GitHub session and highlights recent activity.
        </p>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div class="rounded-2xl border border-black/10 bg-white/90 p-4">
          <p class="text-sm font-semibold text-neutral-800">Template pipeline</p>
          <p class="mt-2 text-xs text-neutral-600">
            Clone, patch ports, and build new stacks from the approved repo.
          </p>
        </div>
        <div class="rounded-2xl border border-black/10 bg-white/90 p-4">
          <p class="text-sm font-semibold text-neutral-800">Tunnel routes</p>
          <p class="mt-2 text-xs text-neutral-600">
            Review DNS records and ingress rules from one view.
          </p>
        </div>
        <div class="rounded-2xl border border-black/10 bg-white/90 p-4">
          <p class="text-sm font-semibold text-neutral-800">Quick services</p>
          <p class="mt-2 text-xs text-neutral-600">
            Point a subdomain at any local port in seconds.
          </p>
        </div>
        <div class="rounded-2xl border border-black/10 bg-white/90 p-4">
          <p class="text-sm font-semibold text-neutral-800">Job timeline</p>
          <p class="mt-2 text-xs text-neutral-600">
            Track live logs for every deploy and teardown job.
          </p>
        </div>
      </div>
    </div>

    <aside class="space-y-4 rounded-[28px] border border-black/10 bg-white/80 p-6">
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-semibold text-neutral-900">Operator</h2>
        <span class="rounded-full border border-black/10 bg-white/70 px-3 py-1 text-[11px] uppercase tracking-[0.2em] text-neutral-500">
          Session
        </span>
      </div>
      <p class="text-sm text-neutral-700">{{ greeting }}</p>

      <div
        v-if="auth.error"
        class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-700"
      >
        {{ auth.error }}
      </div>

      <div
        v-if="auth.loading"
        class="rounded-2xl border border-dashed border-black/10 bg-white/70 p-4 text-xs text-neutral-500"
      >
        Looking for your session cookie...
      </div>

      <div
        v-else-if="auth.user"
        class="flex items-center gap-4 rounded-2xl border border-black/10 bg-white/90 p-4"
      >
        <img
          :src="auth.user.avatarUrl"
          :alt="auth.user.login"
          class="h-12 w-12 rounded-2xl object-cover"
        />
        <div>
          <p class="text-sm font-semibold text-neutral-900">@{{ auth.user.login }}</p>
          <p class="text-xs text-neutral-500">
            Session valid until {{ new Date(auth.user.expiresAt).toLocaleString() }}
          </p>
        </div>
      </div>

      <div
        v-else
        class="rounded-2xl border border-black/10 bg-white/70 p-4 text-sm text-neutral-600"
      >
        Sign in to unlock deployment actions and project lists.
      </div>

      <RouterLink
        v-if="!auth.user"
        to="/login"
        class="inline-flex w-full items-center justify-center rounded-2xl border border-black/10 bg-[color:var(--accent-soft)] px-4 py-2 text-sm font-semibold text-[color:var(--accent-ink)] transition hover:-translate-y-0.5"
      >
        <span class="flex items-center gap-2">
          <NavIcon name="login" class="h-4 w-4" />
          Go to login
        </span>
      </RouterLink>
    </aside>
  </section>
</template>
