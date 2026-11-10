<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Divider from 'primevue/divider'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Password from 'primevue/password'
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
  <div class="signin">
    <div class="panel">
      <div class="brand">
        <span class="logo" aria-hidden="true"><i class="pi pi-video" /></span>
        <h1>WCS Workshops</h1>
        <p class="muted">
          {{ mode === 'create' ? 'Create an account to see the videos shared with you.' : 'Sign in to see the videos shared with you.' }}
        </p>
      </div>

      <form @submit.prevent="submitEmail">
        <div v-if="mode === 'create'" class="field">
          <label for="signin-name">Name (optional)</label>
          <InputText id="signin-name" v-model="name" autocomplete="name" fluid />
        </div>
        <div class="field">
          <label for="signin-email">Email</label>
          <InputText id="signin-email" v-model="email" type="email" autocomplete="email" required fluid />
        </div>
        <div class="field">
          <div style="display: flex; justify-content: space-between; align-items: baseline">
            <label for="signin-password" style="font-size: 0.875rem; font-weight: 500">Password</label>
            <Button
              v-if="mode === 'signIn'"
              label="Forgot password?"
              link
              size="small"
              style="padding: 0"
              :disabled="busy"
              @click="resetPassword"
            />
          </div>
          <Password
            v-model="password"
            input-id="signin-password"
            :feedback="mode === 'create'"
            toggle-mask
            fluid
            :input-props="{
              autocomplete: mode === 'create' ? 'new-password' : 'current-password',
              required: true,
              minlength: mode === 'create' ? 6 : undefined,
            }"
          />
        </div>
        <Button type="submit" :label="mode === 'create' ? 'Create account' : 'Sign in'" fluid :loading="busy" />
      </form>

      <p class="switch">
        <template v-if="mode === 'signIn'">
          New here? <Button label="Create an account" link style="padding: 0" @click="switchMode('create')" />
        </template>
        <template v-else>
          Have an account? <Button label="Sign in" link style="padding: 0" @click="switchMode('signIn')" />
        </template>
      </p>

      <Message v-if="error" severity="error" class="mb">{{ error }}</Message>
      <Message v-if="notice" severity="success" class="mb">{{ notice }}</Message>
      <Message v-if="auth.error" severity="error" size="small" class="mb">{{ auth.error }}</Message>

      <Divider align="center"><span class="hint">or continue with</span></Divider>

      <div class="stack">
        <Button label="Google" icon="pi pi-google" severity="secondary" outlined fluid :disabled="busy" @click="go('google')" />
        <Button label="Apple" icon="pi pi-apple" severity="secondary" outlined fluid :disabled="busy" @click="go('apple')" />
        <Button
          label="Microsoft"
          icon="pi pi-microsoft"
          severity="secondary"
          outlined
          fluid
          :disabled="busy"
          @click="go('microsoft')"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.signin {
  max-width: 400px;
  margin: 6vh auto 0;
}

.brand {
  text-align: center;
  margin-bottom: 20px;
}

.brand h1 {
  font-size: 1.75rem;
  margin: 12px 0 6px;
}

.logo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border-radius: 18px;
  background: var(--p-primary-color);
  color: var(--p-primary-contrast-color);
}

.logo .pi {
  font-size: 1.75rem;
}

.panel {
  padding: 24px 20px;
  background: var(--p-content-background);
  border: 1px solid var(--p-content-border-color);
  border-radius: 16px;
  box-shadow: var(--card-shadow);
}

.switch {
  margin: 14px 0 12px;
  text-align: center;
  font-size: 0.875rem;
  color: var(--p-text-muted-color);
}
</style>
