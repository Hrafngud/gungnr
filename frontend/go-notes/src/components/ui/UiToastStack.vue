<script setup lang="ts">
import { computed } from 'vue'
import { useToastStore } from '@/stores/toasts'

const toastStore = useToastStore()

const toastClass = (tone: string) => `toast toast-${tone}`

const orderedToasts = computed(() => toastStore.toasts)
</script>

<template>
  <transition-group name="toast-slide" tag="div" class="toast-stack">
    <div v-for="toast in orderedToasts" :key="toast.id" :class="toastClass(toast.tone)">
      <div class="toast-body">
        <p class="toast-title">{{ toast.title }}</p>
        <p v-if="toast.message" class="toast-message">{{ toast.message }}</p>
      </div>
      <button
        type="button"
        class="toast-close"
        aria-label="Dismiss notification"
        @click="toastStore.remove(toast.id)"
      >
        x
      </button>
    </div>
  </transition-group>
</template>
