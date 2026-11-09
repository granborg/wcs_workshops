<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { PlaybackInfo, VideoSummary } from '@wcs/shared'
import { api, errorMessage } from '@/services/api'
import Icon from '@/components/Icon.vue'

const route = useRoute()
const router = useRouter()
const videoId = String(route.params.videoId)

const video = ref<VideoSummary | null>(null)
const playback = ref<PlaybackInfo | null>(null)
const error = ref<string | null>(null)
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
  }
}

onMounted(load)
onBeforeUnmount(() => {
  if (poll) clearTimeout(poll)
})
</script>

<template>
  <div class="page-head">
    <button type="button" class="iconbtn" aria-label="Back" @click="router.back()"><Icon name="back" /></button>
    <h2>{{ video?.title ?? '' }}</h2>
  </div>

  <p v-if="error" class="error">{{ error }}</p>

  <div v-if="playback" class="player">
    <iframe
      :src="playback.embedUrl"
      :title="video?.title ?? 'Video'"
      allow="autoplay; fullscreen; picture-in-picture; encrypted-media"
      allowfullscreen
    />
  </div>

  <p v-if="video && video.status !== 'ready'" class="muted" style="margin-top: 12px">
    <template v-if="video.status === 'failed'">This video failed to process on Vimeo.</template>
    <template v-else>Vimeo is still processing this video. Playback starts when it is ready.</template>
  </p>
</template>
