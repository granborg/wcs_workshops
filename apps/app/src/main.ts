import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import { installDeepLinks } from './services/deeplinks'
import './style.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
installDeepLinks(router)
app.mount('#app')
