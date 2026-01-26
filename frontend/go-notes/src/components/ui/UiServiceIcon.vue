<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  type?: string
}>()

const isUrl = computed(() => props.type?.startsWith('http'))
</script>

<template>
  <!-- Render SVG from URL -->
  <img
    v-if="isUrl"
    :src="type"
    alt="service icon"
    class="h-5 w-5"
  />
  <!-- Fallback to inline SVG paths for named icon types -->
  <svg
    v-else
    class="h-5 w-5 text-[color:var(--accent-ink)]"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.6"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <!-- Custom -->
    <path v-if="!type || type === 'custom'" d="M12 2v20m0-20a2 2 0 0 1 2 2m-2-2a2 2 0 0 0-2 2m0 0v16m0 0a2 2 0 0 0 2 2m-2-2a2 2 0 0 1-2-2m4 0a2 2 0 0 1-2 2m0 0H8m12-10H4" />
    <!-- Draw -->
    <path v-else-if="type === 'draw'" d="M3 17v3a1 1 0 0 0 1 1h3m13-4v3a1 1 0 0 1-1 1h-3M3 7V4a1 1 0 0 1 1-1h3m13 4V4a1 1 0 0 0-1-1h-3m-4 15l-3-3m0 0l-3-3m3 3V9m0 0l-3 3m3-3l3 3" />
    <!-- AI -->
    <path v-else-if="type === 'ai'" d="M9.5 2A.5.5 0 0 1 10 2.5V4h4v-1.5a.5.5 0 0 1 1 0V4h3.5a2 2 0 0 1 2 2v3.5a.5.5 0 0 1-1 0V6H5v12.5a.5.5 0 0 1-1 0V6a2 2 0 0 1 2-2H9.5V2.5a.5.5 0 0 1 .5-.5zm3.5 9l3 6m0 0l3-6m-3 6l-3-6" />
    <!-- Server -->
    <path v-else-if="type === 'server'" d="M3 6a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v2a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6zm0 10a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v2a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-2zm4-8h.01M7 17h.01" />
    <!-- Database -->
    <path v-else-if="type === 'database'" d="M12 2C6.48 2 2 3.79 2 6v12c0 2.21 4.48 4 10 4s10-1.79 10-4V6c0-2.21-4.48-4-10-4zm0 18c-4.41 0-8-1.34-8-3v-2.17c1.94.95 4.76 1.67 8 1.67s6.06-.72 8-1.67V17c0 1.66-3.59 3-8 3zm0-7c-4.41 0-8-1.34-8-3V7.83C5.94 8.78 8.76 9.5 12 9.5s6.06-.72 8-1.67V10c0 1.66-3.59 3-8 3z" />
    <!-- Cache -->
    <path v-else-if="type === 'cache'" d="M12 2L2 7v10l10 5 10-5V7L12 2zm0 18l-8-4V8.5l8 4v7.5zm8-4l-8 4v-7.5l8-4V16z" />
    <!-- Search -->
    <template v-else-if="type === 'search'">
      <circle cx="11" cy="11" r="8" />
      <path d="m21 21-4.35-4.35" />
    </template>
    <!-- Storage -->
    <template v-else-if="type === 'storage'">
      <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
      <path d="M3.27 6.96L12 12.01l8.73-5.05M12 22.08V12" />
    </template>
    <!-- Mail -->
    <template v-else-if="type === 'mail'">
      <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" />
      <path d="m22 6-10 7L2 6" />
    </template>
    <!-- Tool -->
    <path v-else-if="type === 'tool'" d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
    <!-- Admin -->
    <path v-else-if="type === 'admin'" d="M12 2L2 7v5c0 6.5 4.5 11 10 11s10-4.5 10-11V7l-10-5zm0 18c-4.41 0-8-3.14-8-7V8.31l8-4 8 4V13c0 3.86-3.59 7-8 7z" />
    <!-- Code -->
    <path v-else-if="type === 'code'" d="M16 18l6-6-6-6M8 6l-6 6 6 6" />
    <!-- Git -->
    <template v-else-if="type === 'git'">
      <path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4" />
      <path d="M9 18c-4.51 2-5-2-7-2" />
    </template>
  </svg>
</template>
