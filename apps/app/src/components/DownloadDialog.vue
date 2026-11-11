<script setup lang="ts">
// The sizes of one video file that the owner or an admin can download. Vimeo makes the files; the links expire,
// so the dialog asks for new links each time it opens.
import { ref, watch } from 'vue'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Skeleton from 'primevue/skeleton'
import type { VideoDownload } from '@/types'
import { api, errorMessage } from '@/services/api'
import { formatBytes } from '@/utils/format'
import { useDelayedFlag } from '@/composables/useDelayedFlag'
import BaseDialog from './BaseDialog.vue'

const props = defineProps<{ open: boolean; videoId: string | null; name: string }>()
const emit = defineEmits<{ close: [] }>()

const files = ref<VideoDownload[]>([])
const loading = ref(false)
const slow = useDelayedFlag(loading)
const error = ref<string | null>(null)

async function load() {
  if (!props.videoId) return
  loading.value = true
  error.value = null
  files.value = []
  try {
    files.value = await api.videos.downloads(props.videoId)
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

function detail(f: VideoDownload): string {
  const parts: string[] = []
  if (f.width && f.height) parts.push(`${f.width} × ${f.height}`)
  if (f.sizeBytes) parts.push(formatBytes(f.sizeBytes))
  return parts.join(' · ')
}
</script>

<template>
  <BaseDialog :open="open" :title="`Download ${name}`" @close="emit('close')">
    <ul v-if="slow" class="list">
      <li v-for="i in 3" :key="i" class="row">
        <div class="grow"><Skeleton width="40%" class="mb" /><Skeleton width="60%" height="0.75rem" /></div>
      </li>
    </ul>

    <Message v-else-if="error" severity="error">{{ error }}</Message>

    <template v-else-if="!loading">
      <p v-if="files.length === 0" class="hint">
        Vimeo gave no download for this video. Vimeo gives download files only to some plans. On the free plan it
        gave none.
      </p>
      <ul v-else class="list">
        <li v-for="f in files" :key="f.url" class="row">
          <div class="grow">
            <div class="title">{{ f.label }}</div>
            <div class="sub">{{ detail(f) }}</div>
          </div>
          <!-- The link goes to Vimeo, which sends the file. A new tab keeps the app open. -->
          <Button
            as="a"
            :href="f.url"
            target="_blank"
            rel="noopener"
            download
            icon="pi pi-download"
            label="Download"
            severity="secondary"
            outlined
            size="small"
          />
        </li>
      </ul>
      <p v-if="files.length > 0" class="hint" style="margin: 0">
        The links work for a short time only. If a download does not start, open this dialog again.
      </p>
    </template>

    <div class="btn-row">
      <Button label="Close" severity="secondary" text @click="emit('close')" />
    </div>
  </BaseDialog>
</template>
