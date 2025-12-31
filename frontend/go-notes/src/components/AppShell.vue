<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import NavIcon from '@/components/NavIcon.vue'

type NavItem = {
  label: string
  to: string
  icon: 'home' | 'overview' | 'activity' | 'logs' | 'host' | 'network' | 'github'
  helper: string
}

const navItems: NavItem[] = [
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
]

const route = useRoute()
const auth = useAuthStore()
const SIDEBAR_KEY = 'warp-panel.sidebar'
const sidebarMode = ref<'expanded' | 'collapsed' | 'hidden'>('expanded')

const isSidebarCollapsed = computed(() => sidebarMode.value === 'collapsed')
const isSidebarHidden = computed(() => sidebarMode.value === 'hidden')

const toggleCollapse = () => {
  sidebarMode.value = isSidebarCollapsed.value ? 'expanded' : 'collapsed'
}

const toggleHidden = () => {
  sidebarMode.value = isSidebarHidden.value ? 'expanded' : 'hidden'
}

onMounted(() => {
  if (typeof window === 'undefined') return
  const stored = window.localStorage.getItem(SIDEBAR_KEY)
  if (stored === 'expanded' || stored === 'collapsed' || stored === 'hidden') {
    sidebarMode.value = stored
  }
})

watch(sidebarMode, (value) => {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(SIDEBAR_KEY, value)
})

const pageTitle = computed(() => {
  const title = route.meta?.title as string | undefined
  return title ?? 'Warp Panel'
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
        class="sticky top-0 hidden h-screen flex-col gap-6 border-r border-[color:var(--border)] bg-[color:var(--surface)] py-8 lg:flex"
        :class="[
          isSidebarHidden ? 'lg:hidden' : 'lg:flex',
          isSidebarCollapsed ? 'w-20 px-3' : 'w-72 px-6',
        ]"
      >
        <div class="flex items-center gap-3">
          <div class="grid h-12 w-12 place-items-center rounded-2xl bg-[color:var(--surface-3)] text-lg font-semibold text-[color:var(--accent-ink)]">
            WP
          </div>
          <div v-if="!isSidebarCollapsed">
            <p class="text-xs uppercase tracking-[0.35em] text-[color:var(--muted-2)]">
              Warp Panel
            </p>
            <p class="text-sm font-semibold text-[color:var(--text)]">
              Host control surface
            </p>
          </div>
        </div>

        <nav class="space-y-2">
          <RouterLink
            v-for="item in navItems"
            :key="item.to"
            :to="item.to"
            class="group flex items-center gap-3 rounded-2xl px-3 py-2 text-sm font-semibold transition"
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

        <div class="mt-auto space-y-3">
          <div
            class="space-y-3 rounded-2xl border border-[color:var(--border)] bg-[color:var(--surface-2)] p-4 text-xs text-[color:var(--muted)]"
            :class="isSidebarCollapsed ? 'p-3' : 'p-4'"
          >
            <p v-if="!isSidebarCollapsed" class="text-[11px] uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Session
            </p>
            <div v-if="auth.user" class="flex items-center gap-3">
              <img
                :src="auth.user.avatarUrl"
                :alt="auth.user.login"
                class="h-9 w-9 rounded-xl object-cover"
              />
              <div v-if="!isSidebarCollapsed">
                <p class="text-sm font-semibold text-[color:var(--text)]">
                  @{{ auth.user.login }}
                </p>
                <p class="text-[11px] text-[color:var(--muted-2)]">
                  Session active
                </p>
              </div>
            </div>
            <p v-else-if="!isSidebarCollapsed" class="text-[color:var(--muted-2)]">
              Sign in to unlock deploy actions.
            </p>
          </div>

          <div class="grid gap-2">
            <button
              type="button"
              class="btn btn-ghost flex w-full items-center justify-center px-3 py-2 text-[11px] font-semibold"
              :title="isSidebarCollapsed ? 'Expand navigation' : 'Collapse navigation'"
              :disabled="isSidebarHidden"
              :aria-label="isSidebarCollapsed ? 'Expand navigation' : 'Collapse navigation'"
              @click="toggleCollapse"
            >
              <span
                v-if="isSidebarCollapsed"
                aria-hidden="true"
                class="text-lg leading-none"
              >
                >>
              </span>
              <span v-else>Collapse nav</span>
            </button>
            <button
              type="button"
              class="btn btn-ghost flex w-full items-center justify-center px-3 py-2 text-[11px] font-semibold"
              :title="isSidebarHidden ? 'Show navigation' : 'Hide navigation'"
              :aria-label="isSidebarHidden ? 'Show navigation' : 'Hide navigation'"
              @click="toggleHidden"
            >
              <span
                v-if="isSidebarCollapsed"
                aria-hidden="true"
                class="text-lg leading-none"
              >
                {{ isSidebarHidden ? '>>' : 'x' }}
              </span>
              <span v-else>{{ isSidebarHidden ? 'Show nav' : 'Hide nav' }}</span>
            </button>
          </div>
        </div>
      </aside>

      <div class="min-h-screen flex-1">
        <button
          v-if="isSidebarHidden"
          type="button"
          class="btn btn-ghost fixed left-4 top-24 z-30 hidden items-center gap-2 px-3 py-2 text-[11px] font-semibold lg:flex"
          @click="toggleHidden"
        >
          Show nav
        </button>
        <header class="sticky top-0 z-20 border-b border-[color:var(--border)] bg-[color:var(--bg-soft)] px-4 py-4">
          <div class="mx-auto flex w-full max-w-7xl items-center justify-between">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                {{ pageTitle }}
              </p>
              <h1 class="text-lg font-semibold text-[color:var(--text)]">
                Warp Panel
              </h1>
            </div>
            <div class="flex items-center gap-3">
              <span class="badge status-neutral">Host ready</span>
              <button
                v-if="auth.user"
                type="button"
                class="btn btn-ghost px-4 py-2 text-xs font-semibold"
                @click="auth.logout"
              >
                Sign out
              </button>
              <RouterLink
                v-else
                to="/login"
                class="btn btn-primary px-4 py-2 text-xs font-semibold"
              >
                Sign in
              </RouterLink>
            </div>
          </div>

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

        <main class="mx-auto w-full max-w-7xl px-4 pb-16 pt-8 sm:px-5">
          <slot />
        </main>
      </div>
    </div>
  </div>
</template>
