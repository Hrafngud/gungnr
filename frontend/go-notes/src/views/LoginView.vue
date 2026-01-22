<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import NavIcon from '@/components/NavIcon.vue'
import UiHeroMark from '@/components/ui/UiHeroMark.vue'

const auth = useAuthStore()
const router = useRouter()
const loginHref = computed(() => auth.loginUrl())
const popup = ref<Window | null>(null)
const pollHandle = ref<number | null>(null)
const waitingForAuth = ref(false)
const redirecting = ref(false)
const mockClickCount = ref(0)

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
    await redirectHome()
    return
  }

  pollHandle.value = window.setTimeout(pollForAuth, 1000)
}

const openLoginPopup = () => {
  if (auth.user) return
  if (typeof window === 'undefined') return
  if (popup.value && !popup.value.closed) {
    popup.value.focus()
    return
  }

  const width = 520
  const height = 720
  const left = Math.max(window.screenX + (window.outerWidth - width) / 2, 0)
  const top = Math.max(window.screenY + (window.outerHeight - height) / 2, 0)
  const features = `width=${width},height=${height},left=${left},top=${top},resizable=yes,scrollbars=yes`
  const authWindow = window.open(loginHref.value, 'gungnr-auth', features)

  if (!authWindow) {
    window.location.href = loginHref.value
    return
  }

  popup.value = authWindow
  waitingForAuth.value = true
  pollForAuth()
}

const handleLoginClick = () => {
  if (auth.user) return
  if (typeof window === 'undefined') return

  mockClickCount.value += 1
  if (mockClickCount.value >= 5) {
    mockClickCount.value = 0
    const pass = window.prompt('Enter dev pass')
    if (pass === 'banana') {
      auth.enableMockAuth()
      redirectHome()
    }
    return
  }

  openLoginPopup()
}

const redirectHome = async () => {
  if (!auth.user || redirecting.value) return
  redirecting.value = true
  stopPolling()
  if (popup.value && !popup.value.closed) {
    popup.value.close()
  }
  popup.value = null
  await router.replace({ name: 'home' })
}

onMounted(() => {
  if (!auth.initialized && !auth.loading) {
    auth.fetchUser()
  }
})

watch(
  () => auth.user,
  (value) => {
    if (value) {
      redirectHome()
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  stopPolling()
})
</script>

<template>
  <section class="grid min-h-screen w-full lg:grid-cols-2">
    <div class="flex flex-col justify-center items-center gap-8">
      <UiHeroMark class="w-full max-w-[320px] text-[color:var(--accent-strong)]" />
      <h1 class="text-4xl font-semibold leading-tight text-[color:var(--text)] sm:text-5xl">
        Gungnr
      </h1>
      <p class="max-w-xl text-base text-[color:var(--muted)] sm:text-lg">
        Hit the web in minutes!
      </p>
    </div>

    <div class="flex flex-col justify-center gap-10 border-[color:var(--border)] bg-[color:var(--surface)] px-8 py-16 sm:px-12 lg:border-l">
      <div class="mx-auto flex w-full max-w-md flex-col gap-6">
        <div class="flex flex-col items-center">
          <h2 class="text-xl font-semibold text-[color:var(--text)]">Connect your account</h2>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            Login with Github!
          </p>
        </div>
        <div>
          <a
            class="btn btn-primary inline-flex w-full items-center justify-center gap-2 px-4 py-3 text-sm font-semibold"
            :href="loginHref"
            @click.prevent="handleLoginClick"
          >
            <NavIcon name="github" class="h-4 w-4" />
            Continue with GitHub
          </a>
          <p v-if="waitingForAuth" class="mt-3 text-xs text-[color:var(--muted-2)]">
            Waiting for GitHub to finish signing you in...
          </p>
          <p class="mt-4 text-xs text-[color:var(--muted-2)]">
            Need access? Ask the panel owner to add you to the allowlist or org.
          </p>
        </div>
      </div>
    </div>
  </section>
</template>
