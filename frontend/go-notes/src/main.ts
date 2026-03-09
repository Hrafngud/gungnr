import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { library } from '@fortawesome/fontawesome-svg-core'
import { faCloudflare, faDocker, faGithub } from '@fortawesome/free-brands-svg-icons'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import router from './router'
import { queryClient } from '@/services/queryClient'
import './style.css'
import App from './App.vue'

library.add(faCloudflare, faDocker, faGithub)

const app = createApp(App)

app.use(createPinia())
app.use(VueQueryPlugin, { queryClient })
app.use(router)
app.component('FontAwesomeIcon', FontAwesomeIcon)

app.mount('#app')
