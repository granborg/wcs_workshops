<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Upload } from 'tus-js-client'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import ProgressBar from 'primevue/progressbar'
import type { LibraryTree } from '@/types'
import { api, errorMessage } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { flatten } from '@/utils/tree'
import { formatBytes } from '@/utils/format'
import FolderSelect from '@/components/FolderSelect.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const tree = ref<LibraryTree | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const file = ref<File | null>(null)
const title = ref('')
const directoryId = ref(typeof route.query.dir === 'string' ? route.query.dir : '')
const progress = ref(0)
const phase = ref<'idle' | 'reserving' | 'uploading' | 'finishing' | 'done'>('idle')
const error = ref<string | null>(null)

const folders = computed(() =>
  tree.value
    ? flatten(tree.value.root).map((d) => ({ id: d.id, depth: d.depth, label: d.depth === 0 ? tree.value!.creator.name : d.name }))
    : [],
)
const busy = computed(() => phase.value !== 'idle' && phase.value !== 'done')

onMounted(async () => {
  if (!auth.me) return
  try {
    tree.value = await api.library.tree(auth.me.id)
    if (!directoryId.value) directoryId.value = tree.value.root.id
  } catch (err) {
    error.value = errorMessage(err)
  }
})

function onFile(e: Event) {
  const input = e.target as HTMLInputElement
  file.value = input.files?.[0] ?? null
  if (file.value && !title.value) title.value = file.value.name.replace(/\.[^.]+$/, '')
}

async function start() {
  const f = file.value
  if (!f || !title.value.trim() || !directoryId.value) return
  error.value = null
  progress.value = 0
  phase.value = 'reserving'
  try {
    const ticket = await api.videos.ticket(title.value.trim(), directoryId.value, f.size)
    phase.value = 'uploading'
    await new Promise<void>((resolve, reject) => {
      const upload = new Upload(f, {
        uploadUrl: ticket.uploadLink,
        chunkSize: 50 * 1024 * 1024,
        retryDelays: [0, 1000, 3000, 5000],
        onError: reject,
        onProgress: (sent, total) => {
          progress.value = total > 0 ? Math.round((sent / total) * 100) : 0
        },
        onSuccess: () => resolve(),
      })
      upload.start()
    })
    phase.value = 'finishing'
    await api.videos.complete(ticket.videoId)
    phase.value = 'done'
    await router.push({ name: 'collection', params: { creatorId: auth.me!.id }, query: { dir: directoryId.value } })
  } catch (err) {
    error.value = errorMessage(err)
    phase.value = 'idle'
  }
}
</script>

<template>
  <form class="narrow card" @submit.prevent="start">
    <div class="field">
      <label for="file">Video file</label>
      <input id="file" ref="fileInput" type="file" accept="video/*" hidden :disabled="busy" @change="onFile" />
      <button type="button" class="picker" :disabled="busy" @click="fileInput?.click()">
        <i class="pi" :class="file ? 'pi-video' : 'pi-cloud-upload'" aria-hidden="true" />
        <span v-if="file" class="picker-text">
          <strong>{{ file.name }}</strong>
          <span class="hint">{{ formatBytes(file.size) }} · tap to choose another file</span>
        </span>
        <span v-else class="picker-text">
          <strong>Choose a video</strong>
          <span class="hint">From this device's files or camera roll</span>
        </span>
      </button>
    </div>

    <div class="field">
      <label for="title">Title</label>
      <InputText id="title" v-model="title" :disabled="busy" maxlength="120" fluid />
    </div>

    <div class="field">
      <label for="dir">Destination folder</label>
      <FolderSelect v-model="directoryId" :options="folders" input-id="dir" :disabled="busy || folders.length === 0" />
    </div>

    <div v-if="busy" class="field">
      <ProgressBar :value="progress" :mode="phase === 'uploading' ? 'determinate' : 'indeterminate'" style="height: 10px" />
      <span class="hint">
        <template v-if="phase === 'reserving'">Preparing upload…</template>
        <template v-else-if="phase === 'uploading'">Uploading {{ progress }}%</template>
        <template v-else>Finishing…</template>
      </span>
    </div>

    <Message v-if="error" severity="error" class="mb">{{ error }}</Message>

    <Button
      type="submit"
      label="Upload"
      icon="pi pi-upload"
      fluid
      :loading="busy"
      :disabled="!file || !title.trim() || !directoryId"
    />
    <p class="hint" style="margin-top: 12px">
      The file goes directly from this device to Vimeo. Keep the app open until the upload finishes.
    </p>
  </form>
</template>

<style scoped>
.picker {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 18px 16px;
  border: 1.5px dashed var(--p-content-border-color);
  border-radius: var(--radius);
  background: var(--p-content-background);
  color: var(--p-text-color);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s;
}

.picker:hover:not(:disabled) {
  border-color: var(--p-primary-color);
}

.picker:disabled {
  opacity: 0.6;
  cursor: default;
}

.picker .pi {
  font-size: 1.75rem;
  color: var(--p-primary-color);
}

.picker-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.picker-text strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
