<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppShell from '@/components/AppShell.vue'
import UiToastStack from '@/components/ui/UiToastStack.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()

const useShell = computed(() => route.meta?.layout !== 'auth')

onMounted(() => {
  auth.fetchUser()
})
</script>

<template>
  <AppShell v-if="useShell">
    <RouterView />
  </AppShell>

  <main v-else class="min-h-screen px-4 py-16 sm:px-5">
    <RouterView />
  </main>

  <UiToastStack />
</template>
