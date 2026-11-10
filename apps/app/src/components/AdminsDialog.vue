<script setup lang="ts">
// Admins of one folder. Only the collection owner sees this dialog.
import { ref, watch } from 'vue'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import ProgressSpinner from 'primevue/progressspinner'
import type { AdminInfo } from '@/types'
import { api, errorMessage } from '@/services/api'
import { useDelayedFlag } from '@/composables/useDelayedFlag'
import BaseDialog from './BaseDialog.vue'

const props = defineProps<{ open: boolean; directoryId: string | null; name: string; isRoot: boolean }>()
const emit = defineEmits<{ close: [] }>()

const admins = ref<AdminInfo[]>([])
const email = ref('')
const loading = ref(false)
const slow = useDelayedFlag(loading)
const busy = ref(false)
const error = ref<string | null>(null)

async function load() {
  if (!props.directoryId) return
  loading.value = true
  error.value = null
  try {
    admins.value = await api.admins.list(props.directoryId)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      email.value = ''
      void load()
    }
  },
)

async function add() {
  if (!props.directoryId || !email.value.trim()) return
  busy.value = true
  error.value = null
  try {
    const added = await api.admins.add(props.directoryId, email.value.trim())
    if (!admins.value.some((a) => a.id === added.id)) admins.value = [...admins.value, added]
    email.value = ''
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}

async function remove(id: string) {
  busy.value = true
  error.value = null
  try {
    await api.admins.remove(id)
    admins.value = admins.value.filter((a) => a.id !== id)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <BaseDialog :open="open" :title="`Admins of ${name}`" @close="emit('close')">
    <p class="hint">
      Admins can add and remove videos, share QR codes, and revoke access in
      {{ isRoot ? 'your whole collection' : 'this folder and the folders in it' }}. They cannot change folders or add
      other admins. Admins of a parent folder are listed on that folder.
    </p>
    <Message v-if="error" severity="error" class="mb">{{ error }}</Message>

    <form class="field" @submit.prevent="add">
      <label for="admin-email">Add an admin by email</label>
      <div style="display: flex; gap: 8px">
        <InputText
          id="admin-email"
          v-model="email"
          type="email"
          autocomplete="off"
          placeholder="name@example.com"
          :disabled="busy"
          fluid
        />
        <Button type="submit" label="Add" icon="pi pi-user-plus" :loading="busy" :disabled="!email.trim()" />
      </div>
      <span class="hint">The person must have a creator account with this email.</span>
    </form>

    <template v-if="loading">
      <div v-if="slow" class="empty"><ProgressSpinner style="width: 32px; height: 32px" /></div>
    </template>
    <div v-else-if="admins.length === 0" class="empty">
      <i class="pi pi-users" aria-hidden="true" />
      <p>No admins on this folder.</p>
    </div>
    <ul v-else class="list">
      <li v-for="a in admins" :key="a.id" class="row">
        <span class="lead"><i class="pi pi-user-edit" aria-hidden="true" /></span>
        <div class="grow">
          <div class="title">{{ a.user.name }}</div>
          <div v-if="a.email" class="sub">{{ a.email }}</div>
        </div>
        <Button label="Remove" severity="danger" text size="small" :disabled="busy" @click="remove(a.id)" />
      </li>
    </ul>
  </BaseDialog>
</template>
