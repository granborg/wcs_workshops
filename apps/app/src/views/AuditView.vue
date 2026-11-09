<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { AuditEventInfo } from '@/types'
import { api, errorMessage } from '@/services/api'
import { formatDateTime } from '@/utils/format'
import Icon from '@/components/Icon.vue'

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
  <div class="page-head">
    <button type="button" class="iconbtn" aria-label="Back" @click="router.back()"><Icon name="back" /></button>
    <h2>Audit log</h2>
  </div>

  <p v-if="error" class="error">{{ error }}</p>
  <p v-if="!loading && events.length === 0" class="empty">No events yet.</p>

  <ul v-else class="list">
    <li v-for="e in events" :key="e.id" class="row" style="align-items: flex-start">
      <div class="grow">
        <div class="title" style="white-space: normal">{{ describe(e) }}</div>
        <div class="sub">
          {{ formatDateTime(e.at) }}
          <template v-if="e.ip"> · {{ e.ip }}</template>
          <template v-if="e.qrCodeId"> · code {{ e.qrCodeId.slice(0, 8) }}</template>
        </div>
      </div>
    </li>
  </ul>

  <button v-if="!done && events.length > 0" type="button" class="btn block" :disabled="loading" @click="loadMore">
    {{ loading ? 'Loading…' : 'Load more' }}
  </button>
</template>
