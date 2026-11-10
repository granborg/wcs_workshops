<script setup lang="ts">
import { ref, watch } from 'vue'
import Button from 'primevue/button'
import Message from 'primevue/message'
import ProgressSpinner from 'primevue/progressspinner'
import type { AccessGrantInfo } from '@/types'
import { api, errorMessage, type ShareTarget } from '@/services/api'
import { formatDateTime } from '@/utils/format'
import { useDelayedFlag } from '@/composables/useDelayedFlag'
import BaseDialog from './BaseDialog.vue'

const props = defineProps<{ open: boolean; target: ShareTarget | null; name: string }>()
const emit = defineEmits<{ close: [] }>()

const grants = ref<AccessGrantInfo[]>([])
const loading = ref(false)
const slow = useDelayedFlag(loading)
const error = ref<string | null>(null)

async function load() {
  if (!props.target) return
  loading.value = true
  error.value = null
  try {
    grants.value = await api.access.list(props.target)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) void load()
  },
)

async function revoke(id: string) {
  error.value = null
  try {
    await api.access.revoke(id)
    grants.value = grants.value.filter((g) => g.id !== id)
  } catch (err) {
    error.value = errorMessage(err)
  }
}
</script>

<template>
  <BaseDialog :open="open" :title="`Access to ${name}`" @close="emit('close')">
    <p class="hint">People granted access on this item. Access inherited from a parent folder is listed on that folder.</p>
    <Message v-if="error" severity="error" class="mb">{{ error }}</Message>
    <template v-if="loading">
      <div v-if="slow" class="empty"><ProgressSpinner style="width: 32px; height: 32px" /></div>
    </template>
    <div v-else-if="grants.length === 0" class="empty">
      <i class="pi pi-users" aria-hidden="true" />
      <p>No one has direct access.</p>
    </div>
    <ul v-else class="list">
      <li v-for="g in grants" :key="g.id" class="row">
        <span class="lead"><i class="pi pi-user" aria-hidden="true" /></span>
        <div class="grow">
          <div class="title">{{ g.user.name }}</div>
          <div class="sub">Since {{ formatDateTime(g.createdAt) }}</div>
        </div>
        <Button label="Revoke" severity="danger" text size="small" @click="revoke(g.id)" />
      </li>
    </ul>
  </BaseDialog>
</template>
