<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Message from 'primevue/message'
import type { AuditEventInfo } from '@/types'
import { api, errorMessage } from '@/services/api'
import { formatDateTime } from '@/utils/format'

const router = useRouter()
const events = ref<AuditEventInfo[]>([])
const loading = ref(false)
const done = ref(false)
const error = ref<string | null>(null)
const PAGE = 50

async function loadMore() {
  if (loading.value || done.value) return
  loading.value = true
  error.value = null
  try {
    const before = events.value[events.value.length - 1]?.at
    const page = await api.audit.list(PAGE, before)
    events.value.push(...page)
    if (page.length < PAGE) done.value = true
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

onMounted(loadMore)

const ICONS: Record<AuditEventInfo['type'], string> = {
  'access.granted': 'pi-user-plus',
  'access.revoked': 'pi-user-minus',
  'qr.created': 'pi-qrcode',
  'qr.revoked': 'pi-ban',
}

function describe(e: AuditEventInfo): string {
  const what = e.videoId ? 'a video' : 'a folder'
  const who = e.subject?.name ?? 'Someone'
  switch (e.type) {
    case 'access.granted':
      return `${who} gained access to ${what} by scanning a QR code`
    case 'access.revoked':
      return `Access to ${what} was revoked for ${who}`
    case 'qr.created':
      return `QR code generated for ${what}`
    case 'qr.revoked':
      return `QR code revoked for ${what}`
  }
}
</script>

<template>
  <div class="page-head card">
    <Button icon="pi pi-arrow-left" severity="secondary" text rounded aria-label="Back" @click="router.back()" />
    <h2>Audit log</h2>
  </div>

  <Message v-if="error" severity="error" class="mb">{{ error }}</Message>

  <div v-if="!loading && events.length === 0" class="empty">
    <i class="pi pi-history" aria-hidden="true" />
    <p>No events yet.</p>
  </div>

  <ul v-else class="list">
    <li v-for="e in events" :key="e.id" class="row" style="align-items: flex-start">
      <span class="lead"><i class="pi" :class="ICONS[e.type]" aria-hidden="true" /></span>
      <div class="grow">
        <div class="title wrap">{{ describe(e) }}</div>
        <div class="sub">
          {{ formatDateTime(e.at) }}
          <template v-if="e.ip"> · {{ e.ip }}</template>
          <template v-if="e.qrCodeId"> · code {{ e.qrCodeId.slice(0, 8) }}</template>
        </div>
      </div>
    </li>
  </ul>

  <Button
    v-if="!done && events.length > 0"
    label="Load more"
    severity="secondary"
    outlined
    fluid
    :loading="loading"
    @click="loadMore"
  />
</template>
