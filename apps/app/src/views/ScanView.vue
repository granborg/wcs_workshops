<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { RedeemResult } from '@wcs/shared'
import { api, errorMessage } from '@/services/api'
import { scanQrCode, scannerAvailable } from '@/services/scanner'
import Icon from '@/components/Icon.vue'

const route = useRoute()
const router = useRouter()

const code = ref('')
const busy = ref(false)
const error = ref<string | null>(null)
const result = ref<RedeemResult | null>(null)
const canScan = scannerAvailable()

async function redeem(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return
  busy.value = true
  error.value = null
  result.value = null
  try {
    result.value = await api.redeem(trimmed)
    code.value = ''
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}

async function scan() {
  error.value = null
  try {
    const value = await scanQrCode()
    if (value) await redeem(value)
  } catch (err) {
    error.value = errorMessage(err)
  }
}

onMounted(() => {
  if (typeof route.query.code === 'string') void redeem(route.query.code)
})

function openResult() {
  if (!result.value) return
  const query = result.value.directoryId ? { dir: result.value.directoryId } : {}
  if (result.value.videoId) void router.push({ name: 'video', params: { videoId: result.value.videoId } })
  else void router.push({ name: 'collection', params: { creatorId: result.value.creator.id }, query })
}
</script>

<template>
  <div style="max-width: 420px; margin: 0 auto; text-align: center">
    <Icon name="qr" style="width: 64px; height: 64px; margin: 24px 0 8px; color: var(--primary)" />
    <p class="muted">Scan a creator's QR code to add their folder or video to your library.</p>

    <button v-if="canScan" type="button" class="btn primary block" :disabled="busy" @click="scan">
      <Icon name="camera" /> Open camera
    </button>
    <p v-else class="muted small">Camera scanning works in the installed app. In the browser, paste the code below.</p>

    <form style="margin-top: 20px; text-align: left" @submit.prevent="redeem(code)">
      <div class="field">
        <label for="code">Or enter a code</label>
        <input id="code" v-model="code" type="text" autocomplete="off" placeholder="wcsw://redeem/…" />
      </div>
      <button type="submit" class="btn block" :disabled="busy || !code.trim()">Add to library</button>
    </form>

    <p v-if="error" class="error">{{ error }}</p>

    <div v-if="result" class="list" style="margin-top: 20px; padding: 16px; text-align: left">
      <p style="display: flex; align-items: center; gap: 8px">
        <Icon name="check" style="width: 20px; height: 20px; color: var(--primary)" />
        <strong>{{ result.added ? 'Added to your library' : 'Already in your library' }}</strong>
      </p>
      <p class="muted">
        {{ result.creator.name }} shared a {{ result.videoId ? 'video' : 'folder' }} with you.
      </p>
      <button type="button" class="btn primary block" @click="openResult">Open</button>
    </div>
  </div>
</template>
