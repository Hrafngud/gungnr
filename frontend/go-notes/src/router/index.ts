import { createRouter, createWebHistory } from 'vue-router'
import LoginView from '@/views/LoginView.vue'
import HomeView from '@/views/HomeView.vue'
import OverviewView from '@/views/OverviewView.vue'
import HostSettingsView from '@/views/HostSettingsView.vue'
import NetworkingView from '@/views/NetworkingView.vue'
import GitHubView from '@/views/GitHubView.vue'
import JobDetailView from '@/views/JobDetailView.vue'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: { title: 'Sign in', layout: 'auth' },
    },
    {
      path: '/',
      name: 'home',
      component: HomeView,
      meta: { title: 'Home' },
    },
    {
      path: '/overview',
      name: 'overview',
      component: OverviewView,
      meta: { title: 'Overview' },
    },
    {
      path: '/host-settings',
      name: 'host-settings',
      component: HostSettingsView,
      meta: { title: 'Host Settings' },
    },
    {
      path: '/networking',
      name: 'networking',
      component: NetworkingView,
      meta: { title: 'Networking' },
    },
    {
      path: '/github',
      name: 'github',
      component: GitHubView,
      meta: { title: 'GitHub' },
    },
    {
      path: '/jobs/:id',
      name: 'job-detail',
      component: JobDetailView,
      meta: { title: 'Job log' },
    },
    {
      path: '/projects',
      redirect: '/',
    },
    {
      path: '/jobs',
      redirect: '/overview',
    },
    {
      path: '/activity',
      redirect: '/overview',
    },
    {
      path: '/settings',
      redirect: '/host-settings',
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/',
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  if (!auth.initialized) {
    await auth.fetchUser()
  }

  if (to.name === 'login') {
    if (auth.isAuthenticated) {
      return { name: 'home' }
    }
    return true
  }

  if (!auth.isAuthenticated) {
    return { name: 'login' }
  }

  return true
})

router.afterEach((to) => {
  if (to.meta?.title) {
    document.title = `${to.meta.title} • Warp Panel`
  }
})

export default router
