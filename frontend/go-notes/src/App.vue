<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import AppHeader from './components/AppHeader.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()

onMounted(() => {
  auth.fetchUser()
})
</script>

<template>
  <div class="min-h-screen text-neutral-900">
    <AppHeader />
    <main class="mx-auto max-w-6xl px-6 pb-16 pt-10">
      <RouterView />
    </main>
  </div>
</template>
