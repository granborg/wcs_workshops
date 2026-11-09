import { App as CapacitorApp } from '@capacitor/app'
import type { Router } from 'vue-router'
import { QR_SCHEME } from '@/types'

/**
 * When the phone camera scans one of our QR codes and the OS opens the app with the wcsw:// URL,
 * route straight to the scan page with the code filled in.
 * The native projects must register the "wcsw" scheme (see docs/ARCHITECTURE.md).
 */
export function installDeepLinks(router: Router): void {
  void CapacitorApp.addListener('appUrlOpen', ({ url }) => {
    if (url.startsWith(QR_SCHEME)) {
      void router.push({ name: 'scan', query: { code: url } })
    }
  })
}
