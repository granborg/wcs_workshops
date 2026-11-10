<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Skeleton from 'primevue/skeleton'
import type { PlaybackInfo, VideoSummary } from '@/types'
import { api, errorMessage } from '@/services/api'
import { useDelayedFlag } from '@/composables/useDelayedFlag'
import { useVideoUpload } from '@/composables/useVideoUpload'
import UploadProgress from '@/components/UploadProgress.vue'
import VideoFilePicker from '@/components/VideoFilePicker.vue'
import UploadLinkDialog from '@/components/UploadLinkDialog.vue'

const route = useRoute()
const router = useRouter()
const videoId = String(route.params.videoId)

const video = ref<VideoSummary | null>(null)
const playback = ref<PlaybackInfo | null>(null)
const error = ref<string | null>(null)
const loading = ref(true)
const slow = useDelayedFlag(loading)
let poll: ReturnType<typeof setTimeout> | null = null

// The owner or an admin can upload the file to a placeholder, retry an upload that did not finish, or replace a
// failed one. They can also give an upload link to someone else, for example the person who filmed the workshop.
const { phase, progress, busy: uploading, upload } = useVideoUpload()
const file = ref<File | null>(null)
const uploadError = ref<string | null>(null)
const showLinkDialog = ref(false)
const canManage = computed(() => video.value?.canManage ?? false)
const canUpload = computed(
  () => canManage.value && (uploading.value || ['placeholder', 'uploading', 'failed'].includes(video.value!.status)),
)

// Size the player frame to the video, so a portrait video is not letterboxed in a 16:9 frame.
// A portrait frame is capped at 70% of the viewport height and centered. 16:9 until Vimeo reports the size.
const playerStyle = computed(() => {
  const w = video.value?.width ?? 16
  const h = video.value?.height ?? 9
  return {
    aspectRatio: `${w} / ${h}`,
    maxWidth: h > w ? `calc(70vh * ${w} / ${h})` : undefined,
  }
})

async function load() {
  if (poll) clearTimeout(poll)
  poll = null
  error.value = null
  try {
    video.value = await api.videos.get(videoId)
    if (video.value.status === 'ready' || video.value.status === 'processing') {
      playback.value = await api.videos.playback(videoId)
    }
    // Check again until the video is ready. Viewers of a placeholder see the video when the creator uploads it.
    if (['placeholder', 'uploading', 'processing'].includes(video.value.status)) {
      poll = setTimeout(load, 10_000)
    }
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function sendFile() {
  if (!file.value) return
  uploadError.value = null
  try {
    await upload(file.value, (size) => api.videos.ticketFor(videoId, size))
    file.value = null
    await load()
  } catch (err) {
    uploadError.value = errorMessage(err)
  }
}

onMounted(load)
onBeforeUnmount(() => {
  if (poll) clearTimeout(poll)
})
</script>

<template>
  <div class="page-head card">
    <Button icon="pi pi-arrow-left" severity="secondary" text rounded aria-label="Back" @click="router.back()" />
    <h2>{{ video?.title ?? '' }}</h2>
  </div>

  <Message v-if="error" severity="error" class="mb">{{ error }}</Message>

  <div v-if="playback" class="player" :style="playerStyle">
    <iframe
      :src="playback.embedUrl"
      :title="video?.title ?? 'Video'"
      allow="autoplay; fullscreen; picture-in-picture; encrypted-media"
      allowfullscreen
    />
  </div>
  <Skeleton v-else-if="slow" width="100%" :style="{ ...playerStyle, height: 'auto' }" border-radius="10px" />

  <div v-if="video?.status === 'placeholder' && !canManage" class="empty">
    <i class="pi pi-clock" aria-hidden="true" />
    <p><strong>Not uploaded yet</strong></p>
    <p>The creator has not uploaded this video yet. It plays here when it is ready.</p>
  </div>

  <Message v-else-if="video?.status === 'failed'" severity="error" style="margin-top: 12px">
    This video failed to process on Vimeo.
  </Message>
  <Message v-else-if="video?.status === 'uploading' && !uploading" severity="info" icon="pi pi-clock" style="margin-top: 12px">
    This video is uploading. Playback starts when the upload finishes and Vimeo has processed the video.
  </Message>
  <Message v-else-if="video?.status === 'processing'" severity="info" icon="pi pi-clock" style="margin-top: 12px">
    Vimeo is still processing this video. Playback starts when it is ready.
  </Message>

  <form v-if="canUpload" class="card upload" @submit.prevent="sendFile">
    <h3>{{ video?.status === 'placeholder' ? 'Upload the video file' : 'Upload the file again' }}</h3>
    <p class="hint">
      <template v-if="video?.status === 'placeholder'">
        This is a placeholder. People who have access see the video here when you upload it.
      </template>
      <template v-else-if="video?.status === 'failed'">Choose the file again to replace the failed upload.</template>
      <template v-else>If the upload did not finish, choose the file again.</template>
    </p>
    <div class="field">
      <VideoFilePicker v-model="file" :disabled="uploading" />
    </div>
    <UploadProgress v-if="uploading" :phase="phase" :progress="progress" />
    <Message v-if="uploadError" severity="error" class="mb">{{ uploadError }}</Message>
    <Button type="submit" label="Upload" icon="pi pi-upload" fluid :loading="uploading" :disabled="!file" />
    <p class="hint" style="margin: 12px 0 0">Keep the app open until the upload finishes.</p>
    <Button
      label="Upload link for someone else"
      icon="pi pi-cloud-upload"
      severity="secondary"
      outlined
      fluid
      style="margin-top: 14px"
      :disabled="uploading"
      @click="showLinkDialog = true"
    />
  </form>

  <UploadLinkDialog :open="showLinkDialog" :video-id="video?.id ?? null" :name="video?.title ?? ''" @close="showLinkDialog = false" />
</template>

<style scoped>
.upload {
  margin-top: 14px;
}

.upload h3 {
  font-size: 1.0625rem;
}
</style>
