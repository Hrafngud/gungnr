import { createRouter, createWebHistory } from 'vue-router'
import LoginView from '@/views/LoginView.vue'
import DashboardView from '@/views/DashboardView.vue'
import ProjectsView from '@/views/ProjectsView.vue'
import JobsView from '@/views/JobsView.vue'
import JobDetailView from '@/views/JobDetailView.vue'
import ActivityView from '@/views/ActivityView.vue'
import SettingsView from '@/views/SettingsView.vue'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    {
      path: '/',
      name: 'dashboard',
      component: DashboardView,
      meta: { title: 'Warp Panel' },
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: { title: 'Sign in' },
    },
    {
      path: '/projects',
      name: 'projects',
      component: ProjectsView,
      meta: { title: 'Projects' },
    },
    {
      path: '/jobs',
      name: 'jobs',
      component: JobsView,
      meta: { title: 'Jobs' },
    },
    {
      path: '/jobs/:id',
      name: 'job-detail',
      component: JobDetailView,
      meta: { title: 'Job log' },
    },
    {
      path: '/activity',
      name: 'activity',
      component: ActivityView,
      meta: { title: 'Activity' },
    },
    {
      path: '/settings',
      name: 'settings',
      component: SettingsView,
      meta: { title: 'Settings' },
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
      return { name: 'dashboard' }
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
