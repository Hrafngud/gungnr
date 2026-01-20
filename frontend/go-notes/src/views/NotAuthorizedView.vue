<script setup lang="ts">
import { computed } from 'vue'
import NavIcon from '@/components/NavIcon.vue'

const supportEmail = computed(() => {
  if (typeof window === 'undefined') return 'support@warp-panel.local'
  const host = window.location.hostname || 'warp-panel.local'
  return `support@${host}`
})
</script>

<template>
  <section class="grid min-h-screen w-full lg:grid-cols-2">
    <div class="flex flex-col justify-center gap-10 bg-[color:var(--surface-3)] px-8 py-16 sm:px-12">
      <div class="space-y-6">
        <div class="inline-flex items-center gap-2 rounded-full border border-[color:var(--border)] bg-[color:var(--surface-2)] px-3 py-1 text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Warp Panel
        </div>
        <h1 class="text-4xl font-semibold leading-tight text-[color:var(--text)] sm:text-5xl">
          You do not have access to this panel yet.
        </h1>
        <p class="max-w-xl text-base text-[color:var(--muted)] sm:text-lg">
          Your GitHub account is not on the allowlist for this deployment. Ask the panel
          owner to add you, then try again.
        </p>
      </div>
      <div class="grid gap-3 text-sm text-[color:var(--muted)] sm:grid-cols-3">
        <span class="rounded-full border border-[color:var(--border)] bg-[color:var(--surface-2)] px-3 py-1 text-center">
          Access restricted
        </span>
        <span class="rounded-full border border-[color:var(--border)] bg-[color:var(--surface-2)] px-3 py-1 text-center">
          Allowlist only
        </span>
        <span class="rounded-full border border-[color:var(--border)] bg-[color:var(--surface-2)] px-3 py-1 text-center">
          OAuth verified
        </span>
      </div>
    </div>

    <div class="flex flex-col justify-center gap-10 border-[color:var(--border)] bg-[color:var(--surface)] px-8 py-16 sm:px-12 lg:border-l">
      <div class="mx-auto flex w-full max-w-md flex-col gap-6">
        <div>
          <h2 class="text-xl font-semibold text-[color:var(--text)]">Request access</h2>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            If you believe this is a mistake, contact the panel owner and include your
            GitHub username.
          </p>
        </div>
        <div class="space-y-3">
          <RouterLink
            class="btn btn-primary inline-flex w-full items-center justify-center gap-2 px-4 py-3 text-sm font-semibold"
            to="/login"
          >
            <NavIcon name="arrow-left" class="h-4 w-4" />
            Back to login
          </RouterLink>
          <a
            class="btn btn-ghost inline-flex w-full items-center justify-center gap-2 px-4 py-3 text-sm font-semibold"
            :href="`mailto:${supportEmail}`"
          >
            <NavIcon name="users" class="h-4 w-4" />
            Contact support
          </a>
          <p class="text-xs text-[color:var(--muted-2)]">
            Support email: {{ supportEmail }}
          </p>
        </div>
      </div>
    </div>
  </section>
</template>
