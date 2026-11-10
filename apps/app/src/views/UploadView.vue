<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Upload } from 'tus-js-client'
import type { LibraryTree } from '@/types'
import { api, errorMessage } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { flatten } from '@/utils/tree'
import { formatBytes } from '@/utils/format'
import Icon from '@/components/Icon.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const tree = ref<LibraryTree | null>(null)
const file = ref<File | null>(null)
const title = ref('')
const directoryId = ref(typeof route.query.dir === 'string' ? route.query.dir : '')
const progress = ref(0)
const phase = ref<'idle' | 'reserving' | 'uploading' | 'finishing' | 'done'>('idle')
const error = ref<string | null>(null)

const folders = computed(() =>
  tree.value ? flatten(tree.value.root).map((d) => ({ ...d, label: d.depth === 0 ? tree.value!.creator.name : d.name })) : [],
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
  <form style="max-width: 480px; margin: 0 auto" @submit.prevent="start">
    <div class="field">
      <label for="file">Video file</label>
      <input id="file" type="file" accept="video/*" :disabled="busy" @change="onFile" />
      <span v-if="file" class="muted small">{{ file.name }} · {{ formatBytes(file.size) }}</span>
    </div>

    <div class="field">
      <label for="title">Title</label>
      <input id="title" v-model="title" type="text" :disabled="busy" maxlength="120" />
    </div>

    <div class="field">
      <label for="dir">Destination folder</label>
      <select id="dir" v-model="directoryId" :disabled="busy || folders.length === 0">
        <option v-for="d in folders" :key="d.id" :value="d.id">{{ '  '.repeat(d.depth) }}{{ d.label }}</option>
      </select>
    </div>

    <div v-if="busy" style="margin: 16px 0">
      <div class="progress"><div :style="{ width: progress + '%' }" /></div>
      <p class="muted small" style="margin-top: 6px">
        <template v-if="phase === 'reserving'">Preparing upload…</template>
        <template v-else-if="phase === 'uploading'">Uploading {{ progress }}%</template>
        <template v-else>Finishing…</template>
      </p>
    </div>

    <p v-if="error" class="error">{{ error }}</p>

    <button type="submit" class="btn primary block" :disabled="busy || !file || !title.trim() || !directoryId">
      <Icon name="upload" /> Upload
    </button>
    <p class="muted small" style="margin-top: 12px">
      The file goes directly from this device to Vimeo. Keep the app open until the upload finishes.
    </p>
  </form>
</template>
