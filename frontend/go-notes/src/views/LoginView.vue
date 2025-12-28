<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const loginHref = computed(() => auth.loginUrl())
</script>

<template>
  <section class="grid items-center gap-10 lg:grid-cols-[1.15fr,0.85fr]">
    <div class="space-y-6">
      <div class="inline-flex items-center gap-2 rounded-full border border-black/10 bg-white/70 px-3 py-1 text-xs uppercase tracking-[0.3em] text-neutral-700">
        Warp Panel
      </div>
      <h1 class="text-4xl font-semibold leading-tight text-neutral-900 sm:text-5xl">
        Orchestrate deployments, tunnels, and ports without touching the terminal.
      </h1>
      <p class="max-w-xl text-base text-neutral-700 sm:text-lg">
        Sign in with GitHub to unlock your deploy queue, tunnel routing, and template
        workflows. Access is restricted to approved users or org members.
      </p>
      <div class="flex flex-wrap gap-3 text-sm text-neutral-600">
        <span class="rounded-full border border-black/10 bg-white/60 px-3 py-1">GitHub OAuth</span>
        <span class="rounded-full border border-black/10 bg-white/60 px-3 py-1">Session cookies</span>
        <span class="rounded-full border border-black/10 bg-white/60 px-3 py-1">No CLI required</span>
      </div>
    </div>

    <div class="rounded-[28px] border border-black/10 bg-white/80 p-6 shadow-[0_20px_60px_-40px_rgba(0,0,0,0.6)] backdrop-blur">
      <h2 class="text-xl font-semibold text-neutral-900">Connect your account</h2>
      <p class="mt-2 text-sm text-neutral-600">
        We only request read access to confirm your identity and org membership.
      </p>
      <a
        class="mt-6 inline-flex w-full items-center justify-center gap-2 rounded-2xl bg-[color:var(--accent)] px-4 py-3 text-sm font-semibold text-white shadow-lg shadow-[color:var(--accent)]/30 transition hover:-translate-y-0.5"
        :href="loginHref"
      >
        Continue with GitHub
      </a>
      <p class="mt-4 text-xs text-neutral-500">
        Need access? Ask the panel owner to add you to the allowlist or org.
      </p>
    </div>
  </section>
</template>
