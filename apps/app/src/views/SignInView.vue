<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { authService, type Provider } from '@/services/auth'
import { errorMessage } from '@/services/api'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const busy = ref(false)
const error = ref<string | null>(null)
const notice = ref<string | null>(null)

type Mode = 'signIn' | 'create'
const mode = ref<Mode>('signIn')
const email = ref('')
const password = ref('')
const name = ref('')

// Firebase error codes the user can act on. Anything else falls back to the raw message.
const AUTH_ERRORS: Record<string, string> = {
  'auth/invalid-credential': 'The email or password is not correct.',
  'auth/wrong-password': 'The email or password is not correct.',
  'auth/user-not-found': 'The email or password is not correct.',
  'auth/invalid-email': 'Enter a valid email address.',
  'auth/missing-password': 'Enter a password.',
  'auth/email-already-in-use': 'An account with this email already exists. Sign in instead.',
  'auth/weak-password': 'Use a password with at least 6 characters.',
  'auth/too-many-requests': 'Too many attempts. Wait a few minutes and try again.',
  'auth/operation-not-allowed': 'Email sign-in is not turned on for this app.',
  'auth/network-request-failed': 'No network connection. Try again.',
}

function friendly(err: unknown): string {
  const code = (err as { code?: unknown })?.code
  if (typeof code === 'string' && AUTH_ERRORS[code]) return AUTH_ERRORS[code]
  return errorMessage(err)
}

async function run(action: () => Promise<void>) {
  busy.value = true
  error.value = null
  notice.value = null
  try {
    await action()
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/library'
    await router.replace(redirect)
  } catch (err) {
    error.value = friendly(err)
  } finally {
    busy.value = false
  }
}

function go(provider: Provider) {
  return run(() => auth.signIn(provider))
}

function submitEmail() {
  const e = email.value.trim()
  if (mode.value === 'create') return run(() => auth.createAccount(e, password.value, name.value.trim() || undefined))
  return run(() => auth.signInWithEmail(e, password.value))
}

async function resetPassword() {
  error.value = null
  notice.value = null
  const e = email.value.trim()
  if (!e) {
    error.value = 'Enter your email address first.'
    return
  }
  busy.value = true
  try {
    await authService.sendPasswordReset(e)
    notice.value = `If an account exists for ${e}, a reset link is on its way.`
  } catch (err) {
    error.value = friendly(err)
  } finally {
    busy.value = false
  }
}

function switchMode(next: Mode) {
  mode.value = next
  error.value = null
  notice.value = null
}
</script>

<template>
  <div style="max-width: 360px; margin: 10vh auto 0; text-align: center">
    <h1 style="font-size: 28px">WCS Workshops</h1>
    <p class="muted">
      {{ mode === 'create' ? 'Create an account to see the videos shared with you.' : 'Sign in to see the videos shared with you.' }}
    </p>

    <form style="margin-top: 24px; text-align: left" @submit.prevent="submitEmail">
      <div v-if="mode === 'create'" class="field">
        <label for="signin-name">Name (optional)</label>
        <input id="signin-name" v-model="name" type="text" autocomplete="name" />
      </div>
      <div class="field">
        <label for="signin-email">Email</label>
        <input id="signin-email" v-model="email" type="email" autocomplete="email" required />
      </div>
      <div class="field">
        <label for="signin-password">Password</label>
        <input
          id="signin-password"
          v-model="password"
          type="password"
          :autocomplete="mode === 'create' ? 'new-password' : 'current-password'"
          :minlength="mode === 'create' ? 6 : undefined"
          required
        />
      </div>
      <button type="submit" class="btn primary block" :disabled="busy">
        {{ mode === 'create' ? 'Create account' : 'Sign in' }}
      </button>
    </form>

    <p class="small" style="margin-top: 12px">
      <template v-if="mode === 'signIn'">
        <a href="#" @click.prevent="switchMode('create')">Create an account</a>
        ·
        <a href="#" @click.prevent="resetPassword">Forgot password?</a>
      </template>
      <a v-else href="#" @click.prevent="switchMode('signIn')">I already have an account</a>
    </p>

    <p class="muted small" style="margin: 20px 0 10px">or</p>

    <div style="display: flex; flex-direction: column; gap: 10px">
      <button class="btn block" :disabled="busy" @click="go('google')">Continue with Google</button>
      <button class="btn block" :disabled="busy" @click="go('apple')">Continue with Apple</button>
      <button class="btn block" :disabled="busy" @click="go('microsoft')">Continue with Microsoft</button>
    </div>

    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="notice" class="muted">{{ notice }}</p>
    <p v-if="auth.error" class="error small">{{ auth.error }}</p>
  </div>
</template>
