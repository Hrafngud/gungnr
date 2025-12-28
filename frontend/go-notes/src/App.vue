<script setup lang="ts">
import { RouterView } from 'vue-router'
import AppHeader from './components/AppHeader.vue'
</script>

<template>
  <div class="min-h-screen bg-slate-950 text-slate-100">
    <AppHeader />
    <main class="mx-auto max-w-6xl px-6 pb-16 pt-10">
      <RouterView />
    </main>
  </div>
</template>
