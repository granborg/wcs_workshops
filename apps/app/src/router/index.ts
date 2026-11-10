import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    creator?: boolean
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: { name: 'library' } },
    { path: '/sign-in', name: 'sign-in', component: () => import('@/views/SignInView.vue') },
    { path: '/library', name: 'library', component: () => import('@/views/LibraryView.vue'), meta: { title: 'Library' } },
    {
      path: '/library/:creatorId',
      name: 'collection',
      component: () => import('@/views/CollectionView.vue'),
      meta: { title: 'Library' },
    },
    { path: '/video/:videoId', name: 'video', component: () => import('@/views/VideoView.vue'), meta: { title: 'Video' } },
    { path: '/scan', name: 'scan', component: () => import('@/views/ScanView.vue'), meta: { title: 'Scan QR code' } },
    // QR codes and copied share links point here. The scan page redeems ?code= on load, after sign-in if needed.
    { path: '/r/:code', redirect: (to) => ({ name: 'scan', query: { code: String(to.params.code) } }) },
    {
      path: '/upload',
      name: 'upload',
      component: () => import('@/views/UploadView.vue'),
      meta: { title: 'Upload', creator: true },
    },
    { path: '/about', name: 'about', component: () => import('@/views/AboutView.vue'), meta: { title: 'How it works' } },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { title: 'Settings' } },
    {
      path: '/settings/audit',
      name: 'audit',
      component: () => import('@/views/AuditView.vue'),
      meta: { title: 'Audit log', creator: true },
    },
    { path: '/:pathMatch(.*)*', redirect: { name: 'library' } },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.init()
  if (to.name !== 'sign-in' && auth.status !== 'signedIn') {
    return { name: 'sign-in', query: { redirect: to.fullPath } }
  }
  if (to.name === 'sign-in' && auth.status === 'signedIn') {
    return { name: 'library' }
  }
  if (to.meta.creator && !auth.isCreator) {
    return { name: 'library' }
  }
  return true
})
