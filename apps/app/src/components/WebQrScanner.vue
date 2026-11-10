<script setup lang="ts">
// Browser QR scanner: live camera preview plus jsQR decoding. The native app uses ML Kit instead
// (services/scanner.ts). The browser shows its camera permission prompt when this component mounts.
import { onBeforeUnmount, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import { cameraErrorMessage } from '@/services/scanner'

const emit = defineEmits<{ result: [value: string]; cancel: []; error: [message: string] }>()

const video = ref<HTMLVideoElement | null>(null)
const starting = ref(true)
let stream: MediaStream | null = null
let frame = 0
let stopped = false

// Decode a downscaled frame: fast enough on phones, and still sharp enough for a QR code that fills the guide.
const MAX_SIDE = 640

function stop() {
  stopped = true
  cancelAnimationFrame(frame)
  stream?.getTracks().forEach((t) => t.stop())
  stream = null
}

onMounted(async () => {
  try {
    stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: { ideal: 'environment' } }, audio: false })
    if (stopped) return stop()
    const v = video.value
    if (!v) return stop()
    v.srcObject = stream
    await v.play()
    starting.value = false

    const { default: jsQR } = await import('jsqr')
    const canvas = document.createElement('canvas')
    const ctx = canvas.getContext('2d', { willReadFrequently: true })
    if (!ctx) throw new Error('This browser cannot read camera frames.')

    const tick = () => {
      if (stopped) return
      if (v.readyState >= v.HAVE_CURRENT_DATA && v.videoWidth > 0) {
        const scale = Math.min(1, MAX_SIDE / Math.max(v.videoWidth, v.videoHeight))
        const w = Math.round(v.videoWidth * scale)
        const h = Math.round(v.videoHeight * scale)
        canvas.width = w
        canvas.height = h
        ctx.drawImage(v, 0, 0, w, h)
        const code = jsQR(ctx.getImageData(0, 0, w, h).data, w, h, { inversionAttempts: 'dontInvert' })
        if (code?.data) {
          stop()
          emit('result', code.data)
          return
        }
      }
      frame = requestAnimationFrame(tick)
    }
    tick()
  } catch (err) {
    stop()
    emit('error', cameraErrorMessage(err))
  }
})

onBeforeUnmount(stop)

function cancel() {
  stop()
  emit('cancel')
}
</script>

<template>
  <div class="scanner" role="dialog" aria-label="Scan QR code">
    <video ref="video" muted playsinline autoplay />
    <div class="guide" aria-hidden="true" />
    <p class="hint">{{ starting ? 'Starting camera…' : 'Point the camera at the QR code' }}</p>
    <Button class="cancel" label="Cancel" icon="pi pi-times" severity="contrast" rounded @click="cancel" />
  </div>
</template>

<style scoped>
.scanner {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: #000;
  display: flex;
  align-items: center;
  justify-content: center;
}

video {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.guide {
  position: relative;
  width: min(70vw, 70vh, 320px);
  aspect-ratio: 1;
  border: 3px solid rgb(255 255 255 / 0.9);
  border-radius: 16px;
  box-shadow: 0 0 0 100vmax rgb(0 0 0 / 0.45);
}

.scanner .hint {
  position: absolute;
  top: calc(env(safe-area-inset-top) + 24px);
  left: 16px;
  right: 16px;
  margin: 0;
  text-align: center;
  color: #fff;
}

.cancel {
  position: absolute;
  bottom: calc(env(safe-area-inset-bottom) + 24px);
  left: 50%;
  transform: translateX(-50%);
  min-width: 160px;
}
</style>
