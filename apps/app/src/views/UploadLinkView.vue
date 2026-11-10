<script setup lang="ts">
// Opened from an upload link: any signed-in user uploads the file of one video in someone else's collection.
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Skeleton from 'primevue/skeleton'
import type { UploadLinkTarget } from '@/types'
import { api, errorMessage } from '@/services/api'
import { useDelayedFlag } from '@/composables/useDelayedFlag'
import { useVideoUpload } from '@/composables/useVideoUpload'
import UploadProgress from '@/components/UploadProgress.vue'
import VideoFilePicker from '@/components/VideoFilePicker.vue'

const route = useRoute()
const code = String(route.params.code)

const target = ref<UploadLinkTarget | null>(null)
const loading = ref(true)
const slow = useDelayedFlag(loading)
const error = ref<string | null>(null)
const file = ref<File | null>(null)
const finished = ref(false)
const { phase, progress, busy, upload } = useVideoUpload()

onMounted(async () => {
  try {
    target.value = await api.uploadLinks.target(code)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
})

async function send() {
  if (!file.value) return
  error.value = null
  try {
    target.value = await upload(
      file.value,
      (size) => api.uploadLinks.ticket(code, size),
      () => api.uploadLinks.complete(code),
    )
    finished.value = true
  } catch (err) {
    error.value = errorMessage(err)
  }
}
</script>

<template>
  <div class="narrow">
    <Skeleton v-if="slow" height="220px" border-radius="10px" />

    <Message v-if="error && !target" severity="error">
      {{ error }}. Ask the person who sent the link for a new one.
    </Message>

    <div v-else-if="target && finished" class="empty">
      <i class="pi pi-check-circle" aria-hidden="true" />
      <p><strong>Thank you</strong></p>
      <p>
        “{{ target.videoTitle }}” is in {{ target.creatorName }}'s collection now, and in your library. Vimeo processes
        it before it plays.
      </p>
      <Button
        as="router-link"
        :to="{ name: 'video', params: { videoId: target.videoId } }"
        label="Open video"
        icon="pi pi-arrow-right"
        icon-pos="right"
      />
    </div>

    <div v-else-if="target && !target.open" class="empty">
      <i class="pi pi-info-circle" aria-hidden="true" />
      <p><strong>This video already has a file</strong></p>
      <p>The upload link for “{{ target.videoTitle }}” does not accept uploads any more.</p>
    </div>

    <form v-else-if="target" class="card" @submit.prevent="send">
      <h2>Upload “{{ target.videoTitle }}”</h2>
      <p class="hint">
        {{ target.creatorName }} asked you to upload this video to their collection. When the upload finishes, the
        video is added to your library too. {{ target.creatorName }} sees that you uploaded it.
      </p>
      <div class="field">
        <VideoFilePicker v-model="file" :disabled="busy" />
      </div>
      <UploadProgress v-if="busy" :phase="phase" :progress="progress" />
      <Message v-if="error" severity="error" class="mb">{{ error }}</Message>
      <Button type="submit" label="Upload" icon="pi pi-upload" fluid :loading="busy" :disabled="!file" />
      <p class="hint" style="margin: 12px 0 0">
        The file goes directly from this device to Vimeo. Keep the app open until the upload finishes.
      </p>
    </form>
  </div>
</template>
