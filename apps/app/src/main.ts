import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import ToastService from 'primevue/toastservice'
import Tooltip from 'primevue/tooltip'
import 'primeicons/primeicons.css'
import App from './App.vue'
import { router } from './router'
import { initColorMode } from './services/colorMode'
import { installDeepLinks } from './services/deeplinks'
import { theme } from './theme'
import './style.css'

initColorMode()
const app = createApp(App)
app.use(createPinia())
app.use(router)
// PrimeVue 4 is MIT. Version 5 and later need a PrimeUI license key, so stay on 4.x unless that changes.
app.use(PrimeVue, { theme, ripple: true })
app.use(ConfirmationService)
app.use(ToastService)
app.directive('tooltip', Tooltip)
installDeepLinks(router)
app.mount('#app')
