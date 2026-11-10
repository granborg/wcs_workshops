import { App as CapacitorApp } from '@capacitor/app'
import type { Router } from 'vue-router'
import { QR_SCHEME } from '@/types'

/**
 * When the OS opens the app with a share URL, route straight to the scan page with the code filled in.
 * New QR codes encode https://<host>/r/<code>. The OS gives those to the app only after App Links (Android)
 * or Universal Links (iOS) are set up; until then they open in the browser, which also works.
 * Older codes encode wcsw://redeem/<code>, which the native projects register.
 */
export function installDeepLinks(router: Router): void {
  void CapacitorApp.addListener('appUrlOpen', ({ url }) => {
    if (url.startsWith(QR_SCHEME) || new URL(url).pathname.startsWith('/r/')) {
      void router.push({ name: 'scan', query: { code: url } })
    }
  })
}
