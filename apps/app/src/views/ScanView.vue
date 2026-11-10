<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { RedeemResult } from '@/types'
import { api, errorMessage } from '@/services/api'
import { isNativeScanner, scanQrCode, scannerAvailable } from '@/services/scanner'
import Button from 'primevue/button'
import Card from 'primevue/card'
import Divider from 'primevue/divider'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import WebQrScanner from '@/components/WebQrScanner.vue'

const route = useRoute()
const router = useRouter()

const code = ref('')
const busy = ref(false)
const error = ref<string | null>(null)
const result = ref<RedeemResult | null>(null)
const canScan = scannerAvailable()
// Browser only: shows the camera overlay. The native app opens the ML Kit scanner instead.
const webScanning = ref(false)

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
  if (!isNativeScanner()) {
    webScanning.value = true
    return
  }
  try {
    const value = await scanQrCode()
    if (value) await redeem(value)
  } catch (err) {
    error.value = errorMessage(err)
  }
}

function onWebResult(value: string) {
  webScanning.value = false
  void redeem(value)
}

function onWebError(message: string) {
  webScanning.value = false
  error.value = message
}

onMounted(() => {
  if (typeof route.query.code === 'string') {
    void redeem(route.query.code)
    // Drop the code from the address bar so a reload does not redeem again.
    void router.replace({ name: 'scan' })
  }
})

function openResult() {
  if (!result.value) return
  const query = result.value.directoryId ? { dir: result.value.directoryId } : {}
  if (result.value.videoId) void router.push({ name: 'video', params: { videoId: result.value.videoId } })
  else void router.push({ name: 'collection', params: { creatorId: result.value.creator.id }, query })
}
</script>

<template>
  <div class="narrow scan card" style="max-width: 420px; text-align: center">
    <div class="hero" aria-hidden="true"><i class="pi pi-qrcode" /></div>
    <p class="muted">Scan a creator's QR code to add their folder or video to your library.</p>

    <Button v-if="canScan" label="Open camera" icon="pi pi-camera" size="large" fluid :loading="busy" @click="scan" />
    <Message v-else severity="secondary" size="small">This browser cannot use the camera here. Paste the link below.</Message>

    <WebQrScanner v-if="webScanning" @result="onWebResult" @cancel="webScanning = false" @error="onWebError" />

    <Divider align="center"><span class="hint">or paste a link</span></Divider>

    <form style="text-align: left" @submit.prevent="redeem(code)">
      <div class="field">
        <label for="code">Share link or code</label>
        <InputText id="code" v-model="code" autocomplete="off" placeholder="https://…/r/…" fluid />
      </div>
      <Button
        type="submit"
        label="Add to library"
        severity="secondary"
        outlined
        fluid
        :loading="busy"
        :disabled="!code.trim()"
      />
    </form>

    <Message v-if="error" severity="error" style="margin-top: 16px; text-align: left">{{ error }}</Message>

    <Card v-if="result" style="margin-top: 20px; text-align: left">
      <template #title>
        <span style="display: flex; align-items: center; gap: 8px">
          <i class="pi pi-check-circle" style="color: var(--p-green-500)" aria-hidden="true" />
          {{ result.added ? 'Added to your library' : 'Already in your library' }}
        </span>
      </template>
      <template #content>
        <p class="muted" style="margin: 0">
          {{ result.creator.name }} shared a {{ result.videoId ? 'video' : 'folder' }} with you.
        </p>
      </template>
      <template #footer>
        <Button label="Open" icon="pi pi-arrow-right" icon-pos="right" fluid @click="openResult" />
      </template>
    </Card>
  </div>
</template>

<style scoped>
/* The Divider label sits on the card: match the card background. */
.scan {
  --p-divider-content-background: var(--p-content-background);
}

.hero {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 88px;
  height: 88px;
  margin: 16px 0 12px;
  border-radius: 24px;
  background: color-mix(in srgb, var(--p-primary-color) 14%, transparent);
  color: var(--p-primary-color);
}

.hero .pi {
  font-size: 2.75rem;
}
</style>
