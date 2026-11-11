<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import SelectButton from 'primevue/selectbutton'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import type { UserRole } from '@/types'
import { api, errorMessage } from '@/services/api'
import { countVideos } from '@/utils/tree'
import { useAuthStore } from '@/stores/auth'
import { colorMode, type ColorMode } from '@/services/colorMode'

const auth = useAuthStore()
const router = useRouter()
const confirm = useConfirm()
const toast = useToast()

const name = ref(auth.me?.name ?? '')
const role = ref<UserRole>(auth.me?.role ?? 'viewer')
const busy = ref(false)
const error = ref<string | null>(null)

const roles: { label: string; value: UserRole }[] = [
  { label: 'Viewer', value: 'viewer' },
  { label: 'Creator', value: 'creator' },
]

const colorModes: { label: string; value: ColorMode; icon: string }[] = [
  { label: 'Light', value: 'light', icon: 'pi pi-sun' },
  { label: 'Dark', value: 'dark', icon: 'pi pi-moon' },
  { label: 'System', value: 'system', icon: 'pi pi-desktop' },
]

watch(
  () => auth.me,
  (me) => {
    if (me) {
      name.value = me.name
      role.value = me.role
    }
  },
)

async function run(action: () => Promise<unknown>): Promise<boolean> {
  busy.value = true
  error.value = null
  try {
    await action()
    toast.add({ severity: 'success', summary: 'Saved', life: 1500 })
    return true
  } catch (err) {
    error.value = errorMessage(err)
    return false
  } finally {
    busy.value = false
  }
}

function saveName() {
  const n = name.value.trim()
  if (!n || n === auth.me?.name) return
  void run(() => auth.update({ name: n }))
}

function onRoleChange(next: UserRole | null) {
  // SelectButton emits null when the active option is clicked again; keep the current role.
  if (!next || next === auth.me?.role) {
    role.value = auth.me?.role ?? 'viewer'
    return
  }
  if (next === 'creator') {
    void run(() => auth.update({ role: 'creator' }))
    return
  }
  confirm.require({
    header: 'Switch to viewer',
    message:
      'A viewer account cannot hold videos. Your collection must be empty first. Any folders and QR codes are removed. Continue?',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', text: true },
    acceptProps: { label: 'Switch to viewer', severity: 'danger' },
    accept: async () => {
      if (!(await run(() => auth.update({ role: 'viewer' })))) role.value = 'creator'
    },
    reject: () => (role.value = 'creator'),
    onHide: () => (role.value = auth.me?.role ?? role.value),
  })
}

async function signOut() {
  await auth.signOut()
  await router.replace({ name: 'sign-in' })
}

const deleting = ref(false)
const deleteError = ref<string | null>(null)

async function askDeleteAccount() {
  const me = auth.me
  if (!me) return
  deleteError.value = null
  // A creator must delete every video first. Check now, so the reason shows before the confirmation.
  // The server checks again.
  if (auth.isCreator) {
    deleting.value = true
    try {
      const n = countVideos((await api.library.tree(me.id)).root)
      if (n > 0) {
        deleteError.value = `Your collection has ${n} ${n === 1 ? 'video' : 'videos'}. Delete all the videos in your collection before you delete your account.`
        return
      }
    } catch (err) {
      deleteError.value = errorMessage(err)
      return
    } finally {
      deleting.value = false
    }
  }
  confirm.require({
    header: 'Delete account',
    message: auth.isCreator
      ? 'This deletes your account, your folders, QR codes, and upload links, and your access to other collections. You cannot undo this. Continue?'
      : 'This deletes your account and your library. You lose access to all videos that were shared with you. You cannot undo this. Continue?',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', text: true },
    acceptProps: { label: 'Delete account', severity: 'danger' },
    accept: async () => {
      deleting.value = true
      try {
        await auth.deleteAccount()
        await router.replace({ name: 'sign-in' })
      } catch (err) {
        deleteError.value = errorMessage(err)
      } finally {
        deleting.value = false
      }
    },
  })
}
</script>

<template>
  <section v-if="auth.me" class="narrow card">
    <div class="profile">
      <span class="avatar" aria-hidden="true">{{ (auth.me.name || '?').slice(0, 1).toUpperCase() }}</span>
      <div style="min-width: 0">
        <h2 style="margin: 0">{{ auth.me.name }}</h2>
        <p class="hint" style="margin: 0">{{ auth.me.email ?? 'No email on this account' }}</p>
      </div>
    </div>

    <div class="field">
      <label for="name">Name</label>
      <InputText id="name" v-model="name" maxlength="80" :disabled="busy" fluid @blur="saveName" @keydown.enter.prevent="saveName" />
      <span class="hint">Viewers see this name in their library. Creators see it when auditing access.</span>
    </div>

    <div class="field">
      <label id="role-label">Account type</label>
      <SelectButton
        v-model="role"
        :options="roles"
        option-label="label"
        option-value="value"
        :allow-empty="false"
        :disabled="busy"
        aria-labelledby="role-label"
        @update:model-value="onRoleChange"
      />
      <span class="hint">Creators can upload, organize, and share videos.</span>
    </div>

    <div class="field">
      <label id="appearance-label">Appearance</label>
      <SelectButton
        v-model="colorMode"
        :options="colorModes"
        option-label="label"
        option-value="value"
        :allow-empty="false"
        aria-labelledby="appearance-label"
      >
        <template #option="{ option }">
          <i :class="option.icon" aria-hidden="true" />
          <span>{{ option.label }}</span>
        </template>
      </SelectButton>
      <span class="hint">System follows this device's light or dark setting.</span>
    </div>

    <ul v-if="auth.isCreator" class="list">
      <li
        class="row clickable"
        role="button"
        tabindex="0"
        @click="router.push({ name: 'audit' })"
        @keydown.enter="router.push({ name: 'audit' })"
      >
        <span class="lead"><i class="pi pi-history" aria-hidden="true" /></span>
        <div class="grow">
          <div class="title">Audit log</div>
          <div class="sub">Who gained or lost access, and when</div>
        </div>
        <i class="pi pi-chevron-right trail" aria-hidden="true" />
      </li>
    </ul>

    <Message v-if="error" severity="error" class="mb">{{ error }}</Message>

    <Button label="Sign out" icon="pi pi-sign-out" severity="secondary" outlined fluid style="margin-top: 8px" @click="signOut" />

    <div class="danger-zone">
      <Message v-if="deleteError" severity="error" class="mb">{{ deleteError }}</Message>
      <Button
        label="Delete account"
        icon="pi pi-trash"
        severity="danger"
        outlined
        fluid
        :loading="deleting"
        :disabled="busy"
        @click="askDeleteAccount"
      />
      <p class="hint" style="margin: 8px 0 0">
        <template v-if="auth.isCreator">You must delete all the videos in your collection first.</template>
        <template v-else>This removes your account and your library. You cannot undo it.</template>
      </p>
    </div>
  </section>
</template>

<style scoped>
.profile {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 24px;
}

.danger-zone {
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid var(--p-content-border-color);
}

.avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: var(--p-primary-color);
  color: var(--p-primary-contrast-color);
  font-size: 1.375rem;
  font-weight: 600;
}
</style>
