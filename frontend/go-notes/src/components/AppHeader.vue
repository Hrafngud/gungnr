<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
</script>

<template>
  <header class="border-b border-black/10 bg-white/70 backdrop-blur">
    <div class="mx-auto flex w-full max-w-6xl items-center justify-between px-6 py-4">
      <div class="flex items-center gap-4">
        <div
          class="flex h-11 w-11 items-center justify-center rounded-2xl bg-[color:var(--accent)] text-lg font-semibold text-white shadow-lg shadow-[color:var(--accent)]/30"
        >
          WP
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
            Warp Panel
          </p>
          <p class="text-base font-semibold text-neutral-900">
            Deploy orchestration for the host stack
          </p>
        </div>
      </div>
      <nav class="flex items-center gap-3 text-sm font-medium text-neutral-700">
        <RouterLink
          to="/"
          class="rounded-full px-4 py-2 transition hover:bg-black/5"
          active-class="bg-black/5 text-neutral-900"
        >
          Home
        </RouterLink>
        <RouterLink
          to="/overview"
          class="rounded-full px-4 py-2 transition hover:bg-black/5"
          active-class="bg-black/5 text-neutral-900"
        >
          Overview
        </RouterLink>
        <RouterLink
          to="/host-settings"
          class="rounded-full px-4 py-2 transition hover:bg-black/5"
          active-class="bg-black/5 text-neutral-900"
        >
          Host Settings
        </RouterLink>
        <RouterLink
          to="/networking"
          class="rounded-full px-4 py-2 transition hover:bg-black/5"
          active-class="bg-black/5 text-neutral-900"
        >
          Networking
        </RouterLink>
        <RouterLink
          to="/github"
          class="rounded-full px-4 py-2 transition hover:bg-black/5"
          active-class="bg-black/5 text-neutral-900"
        >
          GitHub
        </RouterLink>
        <RouterLink
          v-if="!auth.user"
          to="/login"
          class="rounded-full px-4 py-2 transition hover:bg-black/5"
        >
          Login
        </RouterLink>
        <button
          v-if="auth.user"
          type="button"
          class="rounded-full border border-black/10 bg-white/80 px-4 py-2 text-xs font-semibold text-neutral-700 transition hover:-translate-y-0.5"
          @click="auth.logout"
        >
          Sign out
        </button>
      </nav>
    </div>
  </header>
</template>
