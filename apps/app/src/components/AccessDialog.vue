<script setup lang="ts">
import { ref, watch } from 'vue'
import type { AccessGrantInfo } from '@wcs/shared'
import { api, errorMessage, type ShareTarget } from '@/services/api'
import { formatDateTime } from '@/utils/format'
import BaseDialog from './BaseDialog.vue'

const props = defineProps<{ open: boolean; target: ShareTarget | null; name: string }>()
const emit = defineEmits<{ close: [] }>()

const grants = ref<AccessGrantInfo[]>([])
const loading = ref(false)
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
    <p class="muted small">
      People granted access on this item. Access inherited from a parent folder is listed on that folder.
    </p>
    <p v-if="loading" class="muted">Loading…</p>
    <p v-if="error" class="error">{{ error }}</p>
    <p v-else-if="!loading && grants.length === 0" class="empty">No one has direct access.</p>
    <ul v-else class="list">
      <li v-for="g in grants" :key="g.id" class="row">
        <div class="grow">
          <div class="title">{{ g.user.name }}</div>
          <div class="sub">Since {{ formatDateTime(g.createdAt) }}</div>
        </div>
        <button type="button" class="btn danger" @click="revoke(g.id)">Revoke</button>
      </li>
    </ul>
  </BaseDialog>
</template>
