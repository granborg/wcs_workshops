<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import QRCode from 'qrcode'
import Button from 'primevue/button'
import DatePicker from 'primevue/datepicker'
import Message from 'primevue/message'
import ProgressSpinner from 'primevue/progressspinner'
import Tag from 'primevue/tag'
import { useToast } from 'primevue/usetoast'
import type { QrCodeInfo } from '@/types'
import { api, errorMessage, type ShareTarget } from '@/services/api'
import { formatDateTime } from '@/utils/format'
import { useDelayedFlag } from '@/composables/useDelayedFlag'
import BaseDialog from './BaseDialog.vue'

const props = defineProps<{ open: boolean; target: ShareTarget | null; name: string }>()
const emit = defineEmits<{ close: [] }>()
const toast = useToast()

const codes = ref<QrCodeInfo[]>([])
const loading = ref(false)
const slow = useDelayedFlag(loading)
const error = ref<string | null>(null)
const imageUrl = ref<string | null>(null)
const showNewForm = ref(false)
const expiresAt = ref<Date | null>(null)
const creating = ref(false)

const active = computed(() => codes.value.find((c) => !c.revokedAt && (!c.expiresAt || new Date(c.expiresAt) > new Date())) ?? null)
const others = computed(() => codes.value.filter((c) => c.id !== active.value?.id))

async function load() {
  if (!props.target) return
  loading.value = true
  error.value = null
  try {
    codes.value = await api.share.list(props.target)
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
      showNewForm.value = false
      expiresAt.value = null
      void load()
    }
  },
)

watch(active, async (code) => {
  imageUrl.value = code ? await QRCode.toDataURL(code.payload, { width: 640, margin: 1, errorCorrectionLevel: 'M' }) : null
})

async function create() {
  if (!props.target) return
  creating.value = true
  error.value = null
  try {
    const created = await api.share.create(props.target, expiresAt.value?.toISOString())
    codes.value = [created, ...codes.value]
    showNewForm.value = false
    expiresAt.value = null
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    creating.value = false
  }
}

async function revoke(id: string) {
  error.value = null
  try {
    const updated = await api.share.revoke(id)
    codes.value = codes.value.map((c) => (c.id === id ? updated : c))
  } catch (err) {
    error.value = errorMessage(err)
  }
}

async function copyLink() {
  if (!active.value) return
  try {
    await navigator.clipboard.writeText(active.value.payload)
    toast.add({ severity: 'success', summary: 'Link copied', life: 2000 })
  } catch {
    // Clipboard is blocked (no permission or not a secure context): let the user copy it by hand.
    window.prompt('Copy this link', active.value.payload)
  }
}

function print() {
  document.body.classList.add('print-qr')
  const done = () => {
    document.body.classList.remove('print-qr')
    window.removeEventListener('afterprint', done)
  }
  window.addEventListener('afterprint', done)
  window.print()
}

function status(c: QrCodeInfo): { label: string; severity: 'danger' | 'warn' | 'success' } {
  if (c.revokedAt) return { label: 'Revoked', severity: 'danger' }
  if (c.expiresAt && new Date(c.expiresAt) <= new Date()) return { label: 'Expired', severity: 'warn' }
  return { label: 'Active', severity: 'success' }
}
</script>

<template>
  <BaseDialog :open="open" :title="`Share ${name}`" @close="emit('close')">
    <Message v-if="error" severity="error" class="mb">{{ error }}</Message>
    <template v-if="loading">
      <div v-if="slow" class="empty"><ProgressSpinner style="width: 32px; height: 32px" /></div>
    </template>

    <template v-else-if="active && imageUrl">
      <!-- Print shows only this name and the QR code. -->
      <h2 class="print-only print-title">{{ name }}</h2>
      <img :src="imageUrl" class="qr-image" alt="QR code" />
      <p class="hint no-print" style="text-align: center">
        {{ active.redemptions }} scan{{ active.redemptions === 1 ? '' : 's' }}
        <template v-if="active.expiresAt"> · expires {{ formatDateTime(active.expiresAt) }}</template>
      </p>
      <div class="actions no-print">
        <Button label="Copy link" icon="pi pi-link" severity="secondary" outlined @click="copyLink" />
        <Button label="Print" icon="pi pi-print" severity="secondary" outlined @click="print" />
        <Button label="Revoke" icon="pi pi-ban" severity="danger" text @click="revoke(active.id)" />
      </div>
    </template>

    <div v-else class="empty no-print">
      <i class="pi pi-qrcode" aria-hidden="true" />
      <p>No active QR code. Generate one to share this item.</p>
    </div>

    <div class="no-print" style="margin-top: 16px">
      <Button v-if="!showNewForm" label="Generate new code" icon="pi pi-plus" fluid @click="showNewForm = true" />
      <form v-else @submit.prevent="create">
        <div class="field">
          <label for="share-expiry">Expires (optional)</label>
          <DatePicker
            v-model="expiresAt"
            input-id="share-expiry"
            show-time
            hour-format="12"
            :min-date="new Date()"
            show-icon
            show-button-bar
            placeholder="Never"
            fluid
          />
        </div>
        <div class="btn-row">
          <Button label="Cancel" severity="secondary" text :disabled="creating" @click="showNewForm = false" />
          <Button type="submit" label="Generate" icon="pi pi-qrcode" :loading="creating" />
        </div>
      </form>

      <template v-if="others.length">
        <h3 class="hint" style="margin: 20px 0 8px; font-weight: 600">Earlier codes</h3>
        <ul class="list">
          <li v-for="c in others" :key="c.id" class="row">
            <div class="grow">
              <div class="title small">{{ formatDateTime(c.createdAt) }}</div>
              <div class="sub">{{ c.redemptions }} scan{{ c.redemptions === 1 ? '' : 's' }}</div>
            </div>
            <Tag :value="status(c).label" :severity="status(c).severity" />
            <Button v-if="!c.revokedAt" label="Revoke" severity="danger" text size="small" @click="revoke(c.id)" />
          </li>
        </ul>
      </template>
    </div>
  </BaseDialog>
</template>

<style scoped>
.actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.actions > :last-child {
  grid-column: 1 / -1;
}
</style>
