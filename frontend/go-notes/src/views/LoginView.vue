<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const loginHref = computed(() => auth.loginUrl())
const popup = ref<Window | null>(null)
const pollHandle = ref<number | null>(null)
const waitingForAuth = ref(false)

const stopPolling = () => {
  if (pollHandle.value !== null) {
    window.clearTimeout(pollHandle.value)
    pollHandle.value = null
  }
  waitingForAuth.value = false
}

const pollForAuth = async () => {
  if (!popup.value || popup.value.closed) {
    popup.value = null
    stopPolling()
    return
  }

  await auth.fetchUser()
  if (auth.user) {
    popup.value.close()
    popup.value = null
    stopPolling()
    await router.replace({ name: 'home' })
    return
  }

  pollHandle.value = window.setTimeout(pollForAuth, 1000)
}

const openLoginPopup = () => {
  if (auth.user) return
  if (typeof window === 'undefined') return

  const width = 520
  const height = 720
  const left = Math.max(window.screenX + (window.outerWidth - width) / 2, 0)
  const top = Math.max(window.screenY + (window.outerHeight - height) / 2, 0)
  const features = `width=${width},height=${height},left=${left},top=${top},resizable=yes,scrollbars=yes`
  const authWindow = window.open(loginHref.value, 'warp-panel-auth', features)

  if (!authWindow) {
    window.location.href = loginHref.value
    return
  }

  popup.value = authWindow
  waitingForAuth.value = true
  pollForAuth()
}

onBeforeUnmount(() => {
  stopPolling()
})
</script>

<template>
  <section class="grid items-center gap-10 lg:grid-cols-[1.15fr,0.85fr]">
    <div class="space-y-6">
      <div class="inline-flex items-center gap-2 rounded-full border border-[color:var(--border)] bg-[color:var(--surface)] px-3 py-1 text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
        Warp Panel
      </div>
      <h1 class="text-4xl font-semibold leading-tight text-[color:var(--text)] sm:text-5xl">
        Orchestrate deployments, tunnels, and ports without touching the terminal.
      </h1>
      <p class="max-w-xl text-base text-[color:var(--muted)] sm:text-lg">
        Sign in with GitHub to unlock your deploy queue, tunnel routing, and template
        workflows. Access is restricted to approved users or org members.
      </p>
      <div class="flex flex-wrap gap-3 text-sm text-[color:var(--muted)]">
        <span class="rounded-full border border-[color:var(--border)] bg-[color:var(--surface-2)] px-3 py-1">GitHub OAuth</span>
        <span class="rounded-full border border-[color:var(--border)] bg-[color:var(--surface-2)] px-3 py-1">Session cookies</span>
        <span class="rounded-full border border-[color:var(--border)] bg-[color:var(--surface-2)] px-3 py-1">No CLI required</span>
      </div>
    </div>

    <div class="rounded-2xl border border-[color:var(--border)] bg-[color:var(--surface)] p-6">
      <h2 class="text-xl font-semibold text-[color:var(--text)]">Connect your account</h2>
      <p class="mt-2 text-sm text-[color:var(--muted)]">
        We only request read access to confirm your identity and org membership.
      </p>
      <a
        class="btn btn-primary mt-6 inline-flex w-full items-center justify-center gap-2 px-4 py-3 text-sm font-semibold"
        :href="loginHref"
        @click.prevent="openLoginPopup"
      >
        Continue with GitHub
      </a>
      <p v-if="waitingForAuth" class="mt-3 text-xs text-[color:var(--muted-2)]">
        Waiting for GitHub to finish signing you in...
      </p>
      <p class="mt-4 text-xs text-[color:var(--muted-2)]">
        Need access? Ask the panel owner to add you to the allowlist or org.
      </p>
    </div>
  </section>
</template>
