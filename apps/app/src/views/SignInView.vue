<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import type { Provider } from '@/services/auth'
import { errorMessage } from '@/services/api'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const busy = ref(false)
const error = ref<string | null>(null)

async function go(provider: Provider) {
  busy.value = true
  error.value = null
  try {
    await auth.signIn(provider)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/library'
    await router.replace(redirect)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div style="max-width: 360px; margin: 15vh auto 0; text-align: center">
    <h1 style="font-size: 28px">WCS Workshops</h1>
    <p class="muted">Sign in to see the videos shared with you.</p>

    <div style="display: flex; flex-direction: column; gap: 10px; margin-top: 24px">
      <button class="btn block" :disabled="busy" @click="go('google')">Continue with Google</button>
      <button class="btn block" :disabled="busy" @click="go('apple')">Continue with Apple</button>
      <button class="btn block" :disabled="busy" @click="go('microsoft')">Continue with Microsoft</button>
    </div>

    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="auth.error" class="error small">{{ auth.error }}</p>
  </div>
</template>
