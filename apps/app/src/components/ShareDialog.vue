<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import QRCode from 'qrcode'
import type { QrCodeInfo } from '@/types'
import { api, errorMessage, type ShareTarget } from '@/services/api'
import { formatDateTime } from '@/utils/format'
import BaseDialog from './BaseDialog.vue'
import Icon from './Icon.vue'

const props = defineProps<{ open: boolean; target: ShareTarget | null; name: string }>()
const emit = defineEmits<{ close: [] }>()

const codes = ref<QrCodeInfo[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const imageUrl = ref<string | null>(null)
const showNewForm = ref(false)
const expiresAt = ref('')
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
      expiresAt.value = ''
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
    const iso = expiresAt.value ? new Date(expiresAt.value).toISOString() : undefined
    const created = await api.share.create(props.target, iso)
    codes.value = [created, ...codes.value]
    showNewForm.value = false
    expiresAt.value = ''
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

const copied = ref(false)

async function copyLink() {
  if (!active.value) return
  try {
    await navigator.clipboard.writeText(active.value.payload)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
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

function status(c: QrCodeInfo): { label: string; cls: string } {
  if (c.revokedAt) return { label: 'Revoked', cls: 'danger' }
  if (c.expiresAt && new Date(c.expiresAt) <= new Date()) return { label: 'Expired', cls: 'warn' }
  return { label: 'Active', cls: '' }
}
</script>

<template>
  <BaseDialog :open="open" :title="`Share ${name}`" @close="emit('close')">
    <p v-if="loading" class="muted">Loading…</p>
    <p v-if="error" class="error">{{ error }}</p>

    <template v-if="active && imageUrl">
      <img :src="imageUrl" class="qr-image" alt="QR code" />
      <p class="muted small" style="text-align: center">
        {{ active.redemptions }} scan{{ active.redemptions === 1 ? '' : 's' }}
        <template v-if="active.expiresAt"> · expires {{ formatDateTime(active.expiresAt) }}</template>
      </p>
      <div class="btn-row no-print">
        <button type="button" class="btn" @click="copyLink"><Icon :name="copied ? 'check' : 'link'" /> {{ copied ? 'Copied' : 'Copy link' }}</button>
        <button type="button" class="btn" @click="print"><Icon name="print" /> Print</button>
        <button type="button" class="btn danger" @click="revoke(active.id)">Revoke</button>
      </div>
    </template>
    <p v-else-if="!loading" class="muted no-print">No active QR code. Generate one to share this item.</p>

    <div class="no-print">
      <button v-if="!showNewForm" type="button" class="btn primary block" style="margin-top: 12px" @click="showNewForm = true">
        <Icon name="qr" /> Generate new code
      </button>
      <form v-else style="margin-top: 12px" @submit.prevent="create">
        <div class="field">
          <label for="share-expiry">Expires (optional)</label>
          <input id="share-expiry" v-model="expiresAt" type="datetime-local" />
        </div>
        <div class="btn-row">
          <button type="button" class="btn" :disabled="creating" @click="showNewForm = false">Cancel</button>
          <button type="submit" class="btn primary" :disabled="creating">Generate</button>
        </div>
      </form>

      <template v-if="others.length">
        <h3 class="small muted" style="margin-top: 16px">Earlier codes</h3>
        <ul class="list">
          <li v-for="c in others" :key="c.id" class="row">
            <div class="grow">
              <div class="title small">{{ formatDateTime(c.createdAt) }}</div>
              <div class="sub">{{ c.redemptions }} scan{{ c.redemptions === 1 ? '' : 's' }}</div>
            </div>
            <span class="badge" :class="status(c).cls">{{ status(c).label }}</span>
            <button v-if="!c.revokedAt" type="button" class="btn ghost" @click="revoke(c.id)">Revoke</button>
          </li>
        </ul>
      </template>
    </div>
  </BaseDialog>
</template>
