import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import type { Me, UserRole } from '@/types'
import { api, errorMessage } from '@/services/api'
import { authService, type Provider } from '@/services/auth'

type Status = 'loading' | 'signedOut' | 'signedIn'

export const useAuthStore = defineStore('auth', () => {
  const me = ref<Me | null>(null)
  const status = ref<Status>('loading')
  const error = ref<string | null>(null)
  let initPromise: Promise<void> | null = null

  const isCreator = computed(() => me.value?.role === 'creator')

  async function refresh() {
    me.value = await api.me.get()
    status.value = 'signedIn'
  }

  function init() {
    initPromise ??= (async () => {
      try {
        if (await authService.isSignedIn()) await refresh()
        else status.value = 'signedOut'
      } catch (err) {
        error.value = errorMessage(err)
        status.value = 'signedOut'
      }
    })()
    return initPromise
  }

  async function signIn(provider: Provider) {
    error.value = null
    await authService.signIn(provider)
    await refresh()
  }

  async function signOut() {
    await authService.signOut()
    me.value = null
    status.value = 'signedOut'
  }

  async function update(patch: { name?: string; role?: UserRole }) {
    me.value = await api.me.patch(patch)
  }

  return { me, status, error, isCreator, init, refresh, signIn, signOut, update }
})
