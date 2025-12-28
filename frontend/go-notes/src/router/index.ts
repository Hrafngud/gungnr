import { createRouter, createWebHistory } from 'vue-router'
import NotesView from '@/views/NotesView.vue'
import NoteDetailView from '@/views/NoteDetailView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    {
      path: '/',
      name: 'notes',
      component: NotesView,
      meta: { title: 'Notes' },
    },
    {
      path: '/notes/:id',
      name: 'note-detail',
      component: NoteDetailView,
      props: true,
      meta: { title: 'Note detail' },
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/',
    },
  ],
})

router.afterEach((to) => {
  if (to.meta?.title) {
    document.title = `${to.meta.title} • Go Notes`
  }
})

export default router
