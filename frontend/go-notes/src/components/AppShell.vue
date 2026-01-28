<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import NavIcon from '@/components/NavIcon.vue'
import UiPageOverlay from '@/components/ui/UiPageOverlay.vue'
import { usePageLoadingStore } from '@/stores/pageLoading'
import HostStatusHeader from '@/components/home/HostStatusHeader.vue'

type NavItem = {
  label: string
  to: string
  icon: 'home' | 'overview' | 'activity' | 'logs' | 'host' | 'network' | 'github' | 'users'
  helper: string
}

const baseNavItems: NavItem[] = [
  {
    label: 'Home',
    to: '/',
    icon: 'home',
    helper: 'Quick deploy',
  },
  {
    label: 'Overview',
    to: '/overview',
    icon: 'overview',
    helper: 'Jobs and activity',
  },
  {
    label: 'Jobs',
    to: '/jobs',
    icon: 'activity',
    helper: 'Status + logs',
  },
  {
    label: 'Logs',
    to: '/logs',
    icon: 'logs',
    helper: 'Container output',
  },
  {
    label: 'Host Settings',
    to: '/host-settings',
    icon: 'host',
    helper: 'Runtime config',
  },
  {
    label: 'Networking',
    to: '/networking',
    icon: 'network',
    helper: 'Tunnel health',
  },
  {
    label: 'GitHub',
    to: '/github',
    icon: 'github',
    helper: 'Templates',
  },
  {
    label: 'Users',
    to: '/users',
    icon: 'users',
    helper: 'Access control',
  },
]

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const pageLoading = usePageLoadingStore()
const SIDEBAR_KEY = 'gungnr.sidebar'
const sidebarMode = ref<'expanded' | 'collapsed'>('expanded')
const avatarAnimationClass = ref<string>('')

const isSidebarCollapsed = computed(() => sidebarMode.value === 'collapsed')
const showOverlay = computed(() => !auth.initialized || pageLoading.loading)
const navItems = computed(() => baseNavItems)

const toggleCollapse = () => {
  const isCollapsing = !isSidebarCollapsed.value
  avatarAnimationClass.value = isCollapsing ? 'avatar-collapsing' : 'avatar-expanding'
  setTimeout(() => {
    sidebarMode.value = isSidebarCollapsed.value ? 'expanded' : 'collapsed'
    setTimeout(() => {
      avatarAnimationClass.value = ''
    }, 400)
  }, 50)
}

const handleLogout = async () => {
  await auth.logout()
  router.push('/login')
}

onMounted(() => {
  if (typeof window === 'undefined') return
  const stored = window.localStorage.getItem(SIDEBAR_KEY)
  if (stored === 'expanded' || stored === 'collapsed') {
    sidebarMode.value = stored
  }
})

watch(sidebarMode, (value) => {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(SIDEBAR_KEY, value)
})

const pageTitle = computed(() => {
  const title = route.meta?.title as string | undefined
  return title ?? 'Gungnr'
})

const isActive = (to: string) => {
  if (to === '/') return route.path === '/'
  return route.path.startsWith(to)
}
</script>

<template>
  <div class="min-h-screen text-[color:var(--text)]">
    <div class="flex">
      <aside
        class="sidebar-nav sticky top-0 hidden h-screen flex-col gap-6 border-r border-[color:var(--border)] bg-[color:var(--surface)] py-8 lg:flex transition-all duration-300 ease-out will-change-all"
        :class="[
          isSidebarCollapsed ? 'w-20 px-3' : 'w-72 px-6',
        ]"
      >
        <div
          class="github-badge-container flex flex-col items-center justify-center rounded-[25px] p-2 border border-[color:var(--border)] bg-[color:var(--surface-2)] text-xs text-[color:var(--muted)]"
          :class="[
            isSidebarCollapsed ? 'p-0 h-9 w-9 mx-auto  collapsed' : 'text-center',
          ]"
        >
          <div v-if="auth.user" class="flex flex-col items-center">
            <img
              :src="auth.user.avatarUrl"
              :alt="auth.user.login"
              class="w-8 h-fit rounded-full object-cover"
              :class="avatarAnimationClass"
            />
            <div v-if="!isSidebarCollapsed" class="transition-opacity duration-300 ease-out text-center">
              <p class="text-sm font-semibold text-[color:var(--text)]">
                @{{ auth.user.login }}
              </p>
              <p class="text-[11px] text-[color:var(--muted-2)]">
                GitHub connected
              </p>
            </div>
          </div>
          <p v-else-if="!isSidebarCollapsed" class="text-[color:var(--muted-2)] text-center">
            Sign in to connect GitHub.
          </p>
        </div>

        <nav class="space-y-2">
          <RouterLink
            v-for="item in navItems"
            :key="item.to"
            :to="item.to"
            class="group flex items-center gap-3 rounded-2xl px-3 py-2 text-sm font-semibold transition-all duration-300 ease-out"
            :title="isSidebarCollapsed ? item.label : undefined"
            :class="[
              isActive(item.to)
                ? 'bg-[color:var(--surface-2)] text-[color:var(--text)]'
                : 'text-[color:var(--muted)] hover:bg-[color:var(--surface-2)]',
              isSidebarCollapsed ? 'justify-center' : '',
            ]"
          >
            <NavIcon
              :name="item.icon"
              class="h-4 w-4 text-[color:var(--accent-ink)]"
            />
            <div v-if="!isSidebarCollapsed">
              <p class="text-sm">{{ item.label }}</p>
              <p class="text-[11px] font-medium text-[color:var(--muted-2)]">
                {{ item.helper }}
              </p>
            </div>
          </RouterLink>
        </nav>

        <div class="mt-auto grid place-items-center">
          <button
            type="button"
            class="btn btn-ghost flex w-full items-center justify-center rounded-full p-2"
            :title="isSidebarCollapsed ? 'Expand navigation' : 'Collapse navigation'"
            :aria-label="isSidebarCollapsed ? 'Expand navigation' : 'Collapse navigation'"
            @click="toggleCollapse"
          >
            <NavIcon
              :name="isSidebarCollapsed ? 'arrow-right' : 'arrow-left'"
              class="h-4 w-4 text-[color:var(--muted-2)]"
            />
          </button>
        </div>
      </aside>

      <div class="min-h-screen flex-1">

        <header class="sticky top-0 z-20 border-b border-[color:var(--border)] bg-gradient-to-br from-zinc-600/30 to-zinc-900 px-4 py-4">
          <div class="mx-auto flex w-full max-w-7xl flex-wrap items-center justify-between gap-4">
            <div class="flex flex-row justify-between gap-8 items-center">
              <div class="w-4 h-fit flex items-center">
                <img src="/logo.svg" alt="Gungnr logo">
              </div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                {{ pageTitle }}
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-3">
              <button
                v-if="auth.user"
                type="button"
                class="btn btn-ghost px-4 py-2 text-xs font-semibold"
                @click="handleLogout"
              >
                <span class="flex items-center gap-2">
                  <NavIcon name="logout" class="h-3.5 w-3.5" />
                  Sign out
                </span>
              </button>
              <RouterLink
                v-else
                to="/login"
                class="btn btn-primary px-4 py-2 text-xs font-semibold"
              >
                <span class="flex items-center gap-2">
                  <NavIcon name="login" class="h-3.5 w-3.5" />
                  Sign in
                </span>
              </RouterLink>
            </div>
          </div>
          <HostStatusHeader />

          <nav class="mx-auto mt-4 flex w-full max-w-7xl gap-2 overflow-x-auto pb-2 lg:hidden">
            <RouterLink
              v-for="item in navItems"
              :key="`mobile-${item.to}`"
              :to="item.to"
              class="flex shrink-0 items-center gap-2 rounded-full border border-[color:var(--border)] px-3 py-1 text-xs font-semibold"
              :class="isActive(item.to)
                ? 'bg-[color:var(--surface-2)] text-[color:var(--text)]'
                : 'text-[color:var(--muted)]'"
            >
              <NavIcon :name="item.icon" class="h-3.5 w-3.5" />
              {{ item.label }}
            </RouterLink>
          </nav>
        </header>

        <main class="w-full px-[5%] pb-16 pt-8">
          <slot />
        </main>
      </div>
    </div>

    <UiPageOverlay :show="showOverlay" :message="pageLoading.message" />
  </div>
</template>

<style scoped>
  @keyframes avatarCollapse {
    0% {
      transform: scale(1) rotate(0deg);
      filter: drop-shadow(0 1px 3px rgba(0, 0, 0, 0.1));
    }
    50% {
      transform: scale(1.05) rotate(-2deg);
    }
    100% {
      transform: scale(1) rotate(0deg);
      filter: drop-shadow(0 4px 12px rgba(0, 0, 0, 0.15));
    }
  }

  @keyframes avatarExpand {
    0% {
      transform: scale(1) rotate(0deg);
      filter: drop-shadow(0 4px 12px rgba(0, 0, 0, 0.15));
    }
    50% {
      transform: scale(1.05) rotate(2deg);
    }
    100% {
      transform: scale(1) rotate(0deg);
      filter: drop-shadow(0 1px 3px rgba(0, 0, 0, 0.1));
    }
  }

  .avatar-collapsing {
    animation: avatarCollapse 400ms cubic-bezier(0.34, 1.56, 0.64, 1) forwards;
  }

  .avatar-expanding {
    animation: avatarExpand 400ms cubic-bezier(0.34, 1.56, 0.64, 1) forwards;
  }

  .github-badge-container {
    transition: all 350ms cubic-bezier(0.34, 1.56, 0.64, 1);
  }

  .github-badge-container.collapsed {
    background: linear-gradient(135deg, var(--surface-2), var(--surface));
  }
</style>
