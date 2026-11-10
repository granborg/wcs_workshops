<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Skeleton from 'primevue/skeleton'
import type { PlaybackInfo, VideoSummary } from '@/types'
import { api, errorMessage } from '@/services/api'
import { useDelayedFlag } from '@/composables/useDelayedFlag'

const route = useRoute()
const router = useRouter()
const videoId = String(route.params.videoId)

const video = ref<VideoSummary | null>(null)
const playback = ref<PlaybackInfo | null>(null)
const error = ref<string | null>(null)
const loading = ref(true)
const slow = useDelayedFlag(loading)
let poll: ReturnType<typeof setTimeout> | null = null

async function load() {
  error.value = null
  try {
    video.value = await api.videos.get(videoId)
    if (video.value.status === 'ready' || video.value.status === 'processing') {
      playback.value = await api.videos.playback(videoId)
    }
    if (video.value.status === 'uploading' || video.value.status === 'processing') {
      poll = setTimeout(load, 10_000)
    }
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

onMounted(load)
onBeforeUnmount(() => {
  if (poll) clearTimeout(poll)
})
</script>

<template>
  <div class="page-head">
    <Button icon="pi pi-arrow-left" severity="secondary" text rounded aria-label="Back" @click="router.back()" />
    <h2>{{ video?.title ?? '' }}</h2>
  </div>

  <Message v-if="error" severity="error" class="mb">{{ error }}</Message>

  <div v-if="playback" class="player">
    <iframe
      :src="playback.embedUrl"
      :title="video?.title ?? 'Video'"
      allow="autoplay; fullscreen; picture-in-picture; encrypted-media"
      allowfullscreen
    />
  </div>
  <Skeleton v-else-if="slow" width="100%" style="aspect-ratio: 16 / 9; height: auto" border-radius="10px" />

  <Message v-if="video?.status === 'failed'" severity="error" style="margin-top: 12px">
    This video failed to process on Vimeo.
  </Message>
  <Message v-else-if="video && video.status !== 'ready'" severity="info" icon="pi pi-clock" style="margin-top: 12px">
    Vimeo is still processing this video. Playback starts when it is ready.
  </Message>
</template>
