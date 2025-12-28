<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>

<template>
  <header class="border-b border-white/10 bg-slate-900/80 backdrop-blur">
    <div
      class="mx-auto flex w-full max-w-6xl items-center justify-between px-6 py-4"
    >
      <div class="flex items-center gap-4">
        <div
          class="flex h-11 w-11 items-center justify-center rounded-2xl bg-gradient-to-br from-indigo-500 to-cyan-400 text-lg font-semibold text-white shadow-lg shadow-indigo-500/30"
        >
          GN
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.25em] text-indigo-200">
            Go Notes
          </p>
          <p class="text-base font-semibold text-white">
            Ideas, todos, and research in one place
          </p>
        </div>
      </div>
      <nav class="flex items-center gap-2 text-sm font-medium text-slate-200">
        <RouterLink
          to="/"
          class="rounded-full px-4 py-2 transition hover:bg-white/10"
          active-class="bg-indigo-500/25 text-white shadow-sm shadow-indigo-500/30"
        >
          Notes
        </RouterLink>
        <a
          class="rounded-full px-4 py-2 text-slate-300 transition hover:bg-white/10"
          href="https://github.com"
          target="_blank"
          rel="noreferrer"
        >
          Repo
        </a>
      </nav>
    </div>
  </header>
</template>
