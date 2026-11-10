<script setup lang="ts">
// Upload links for one video with no file yet. Anyone who opens the link, after sign-in, can upload the file.
import { computed, ref, watch } from 'vue'
import QRCode from 'qrcode'
import Button from 'primevue/button'
import Message from 'primevue/message'
import ProgressSpinner from 'primevue/progressspinner'
import { useToast } from 'primevue/usetoast'
import type { UploadLinkInfo } from '@/types'
import { api, errorMessage } from '@/services/api'
import { formatDateTime } from '@/utils/format'
import { useDelayedFlag } from '@/composables/useDelayedFlag'
import BaseDialog from './BaseDialog.vue'

const props = defineProps<{ open: boolean; videoId: string | null; name: string }>()
const emit = defineEmits<{ close: [] }>()
const toast = useToast()

const links = ref<UploadLinkInfo[]>([])
const loading = ref(false)
const slow = useDelayedFlag(loading)
const busy = ref(false)
const error = ref<string | null>(null)
const imageUrl = ref<string | null>(null)

const active = computed(() => links.value.find((l) => !l.revokedAt) ?? null)

async function load() {
  if (!props.videoId) return
  loading.value = true
  error.value = null
  try {
    links.value = await api.uploadLinks.list(props.videoId)
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

watch(active, async (link) => {
  imageUrl.value = link ? await QRCode.toDataURL(link.url, { width: 640, margin: 1, errorCorrectionLevel: 'M' }) : null
})

async function create() {
  if (!props.videoId) return
  busy.value = true
  error.value = null
  try {
    links.value = [await api.uploadLinks.create(props.videoId), ...links.value]
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}

async function revoke(id: string) {
  busy.value = true
  error.value = null
  try {
    const updated = await api.uploadLinks.revoke(id)
    links.value = links.value.map((l) => (l.id === id ? updated : l))
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}

async function copyLink() {
  if (!active.value) return
  try {
    await navigator.clipboard.writeText(active.value.url)
    toast.add({ severity: 'success', summary: 'Link copied', life: 2000 })
  } catch {
    // Clipboard is blocked (no permission or not a secure context): let the user copy it by hand.
    window.prompt('Copy this link', active.value.url)
  }
}
</script>

<template>
  <BaseDialog :open="open" :title="`Upload link for ${name}`" @close="emit('close')">
    <p class="hint">
      Anyone who opens this link, after they sign in, can upload the file of this video. For example, someone who
      filmed the workshop. The link stops working when the video has a file. When the upload finishes, the person
      who uploaded gets access to watch this video. You can revoke it in "View / edit access".
    </p>
    <Message v-if="error" severity="error" class="mb">{{ error }}</Message>
    <template v-if="loading">
      <div v-if="slow" class="empty"><ProgressSpinner style="width: 32px; height: 32px" /></div>
    </template>

    <template v-else-if="active">
      <img v-if="imageUrl" :src="imageUrl" class="qr-image" alt="QR code of the upload link" />
      <p class="hint" style="text-align: center">
        <template v-if="active.usedBy">
          {{ active.usedBy.name }} started an upload {{ formatDateTime(active.usedAt!) }}
        </template>
        <template v-else>Created {{ formatDateTime(active.createdAt) }}. Not used yet.</template>
      </p>
      <div class="btn-row" style="justify-content: center">
        <Button label="Copy link" icon="pi pi-link" severity="secondary" outlined @click="copyLink" />
        <Button label="Revoke" icon="pi pi-ban" severity="danger" text :disabled="busy" @click="revoke(active.id)" />
      </div>
    </template>

    <div v-else class="empty">
      <i class="pi pi-cloud-upload" aria-hidden="true" />
      <p>No active upload link.</p>
      <Button label="Create upload link" icon="pi pi-plus" :loading="busy" @click="create" />
    </div>
  </BaseDialog>
</template>
