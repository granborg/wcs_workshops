<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import type { UserRole } from '@/types'
import { errorMessage } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import Icon from '@/components/Icon.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const auth = useAuthStore()
const router = useRouter()

const name = ref(auth.me?.name ?? '')
const role = ref<UserRole>(auth.me?.role ?? 'viewer')
const busy = ref(false)
const error = ref<string | null>(null)
const saved = ref(false)
const confirmViewer = ref(false)

watch(
  () => auth.me,
  (me) => {
    if (me) {
      name.value = me.name
      role.value = me.role
    }
  },
)

async function run(action: () => Promise<unknown>) {
  busy.value = true
  error.value = null
  saved.value = false
  try {
    await action()
    saved.value = true
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}

function saveName() {
  const n = name.value.trim()
  if (!n || n === auth.me?.name) return
  void run(() => auth.update({ name: n }))
}

function onRoleChange() {
  if (role.value === auth.me?.role) return
  if (role.value === 'viewer') {
    confirmViewer.value = true
    return
  }
  void run(() => auth.update({ role: 'creator' }))
}

function becomeViewer() {
  confirmViewer.value = false
  void run(() => auth.update({ role: 'viewer' })).then(() => {
    if (error.value) role.value = 'creator'
  })
}

function cancelViewer() {
  confirmViewer.value = false
  role.value = 'creator'
}

async function signOut() {
  await auth.signOut()
  await router.replace({ name: 'sign-in' })
}
</script>

<template>
  <section v-if="auth.me" style="max-width: 480px; margin: 0 auto">
    <h2>Account</h2>
    <p class="muted small">Signed in as {{ auth.me.email ?? 'unknown' }}</p>

    <div class="field">
      <label for="name">Name</label>
      <input id="name" v-model="name" type="text" maxlength="80" :disabled="busy" @blur="saveName" @keydown.enter.prevent="saveName" />
      <span class="muted small">Viewers see this name in their library. Creators see it when auditing access.</span>
    </div>

    <div class="field">
      <label for="role">Account type</label>
      <select id="role" v-model="role" :disabled="busy" @change="onRoleChange">
        <option value="viewer">Viewer</option>
        <option value="creator">Creator</option>
      </select>
      <span class="muted small">Creators can upload, organize, and share videos.</span>
    </div>

    <ul v-if="auth.isCreator" class="list">
      <li class="row clickable" role="button" tabindex="0" @click="router.push({ name: 'audit' })" @keydown.enter="router.push({ name: 'audit' })">
        <span class="lead"><Icon name="list" /></span>
        <div class="grow"><div class="title">Audit log</div><div class="sub">Who gained or lost access, and when</div></div>
        <span class="lead"><Icon name="chevron-right" /></span>
      </li>
    </ul>

    <p v-if="error" class="error">{{ error }}</p>
    <p v-else-if="saved" class="muted small">Saved.</p>

    <button type="button" class="btn block" style="margin-top: 24px" @click="signOut"><Icon name="signout" /> Sign out</button>

    <ConfirmDialog
      :open="confirmViewer"
      title="Switch to viewer"
      message="A viewer account cannot hold videos. Your collection must be empty first. Any folders and QR codes are removed. Continue?"
      confirm-label="Switch to viewer"
      danger
      :busy="busy"
      @close="cancelViewer"
      @confirm="becomeViewer"
    />
  </section>
</template>
