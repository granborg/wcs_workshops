<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import SelectButton from 'primevue/selectbutton'
import type { LibraryTree } from '@/types'
import { api, errorMessage } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { useVideoUpload } from '@/composables/useVideoUpload'
import { flatten } from '@/utils/tree'
import FolderSelect from '@/components/FolderSelect.vue'
import UploadProgress from '@/components/UploadProgress.vue'
import VideoFilePicker from '@/components/VideoFilePicker.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const { phase, progress, busy: uploading, upload } = useVideoUpload()

// "now" uploads a file. "later" makes a placeholder: it shows in the library and can be shared at once,
// and the creator uploads the file from the video's page when it is ready.
const modes = [
  { label: 'Upload now', value: 'now' },
  { label: 'Placeholder', value: 'later' },
]
const mode = ref<'now' | 'later'>('now')

const tree = ref<LibraryTree | null>(null)
const file = ref<File | null>(null)
const title = ref('')
const directoryId = ref(typeof route.query.dir === 'string' ? route.query.dir : '')
const creating = ref(false)
const error = ref<string | null>(null)

const folders = computed(() =>
  tree.value
    ? flatten(tree.value.root).map((d) => ({ id: d.id, depth: d.depth, label: d.depth === 0 ? tree.value!.creator.name : d.name }))
    : [],
)
const busy = computed(() => uploading.value || creating.value)
const canSubmit = computed(() => !!title.value.trim() && !!directoryId.value && (mode.value === 'later' || !!file.value))

onMounted(async () => {
  if (!auth.me) return
  try {
    tree.value = await api.library.tree(auth.me.id)
    if (!directoryId.value) directoryId.value = tree.value.root.id
  } catch (err) {
    error.value = errorMessage(err)
  }
})

// Suggest the file name as the title.
watch(file, (f) => {
  if (f && !title.value) title.value = f.name.replace(/\.[^.]+$/, '')
})

async function start() {
  if (!canSubmit.value) return
  error.value = null
  try {
    if (mode.value === 'later') {
      creating.value = true
      await api.videos.placeholder(title.value.trim(), directoryId.value)
    } else {
      const t = title.value.trim()
      await upload(file.value!, (size) => api.videos.ticket(t, directoryId.value, size))
    }
    await router.push({ name: 'collection', params: { creatorId: auth.me!.id }, query: { dir: directoryId.value } })
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <form class="narrow card" @submit.prevent="start">
    <div class="field">
      <SelectButton
        v-model="mode"
        :options="modes"
        option-label="label"
        option-value="value"
        :allow-empty="false"
        :disabled="busy"
        aria-label="Upload now or create a placeholder"
        fluid
      />
      <span v-if="mode === 'later'" class="hint">
        A placeholder shows in your library now, and you can share it. Upload the file later from the video's page.
      </span>
    </div>

    <div v-if="mode === 'now'" class="field">
      <label for="file">Video file</label>
      <VideoFilePicker v-model="file" input-id="file" :disabled="busy" />
    </div>

    <div class="field">
      <label for="title">Title</label>
      <InputText id="title" v-model="title" :disabled="busy" maxlength="120" fluid />
    </div>

    <div class="field">
      <label for="dir">Destination folder</label>
      <FolderSelect v-model="directoryId" :options="folders" input-id="dir" :disabled="busy || folders.length === 0" />
    </div>

    <UploadProgress v-if="uploading" :phase="phase" :progress="progress" />

    <Message v-if="error" severity="error" class="mb">{{ error }}</Message>

    <Button
      type="submit"
      :label="mode === 'later' ? 'Create placeholder' : 'Upload'"
      :icon="mode === 'later' ? 'pi pi-clock' : 'pi pi-upload'"
      fluid
      :loading="busy"
      :disabled="!canSubmit"
    />
    <p v-if="mode === 'now'" class="hint" style="margin: 12px 0 0">
      The file goes directly from this device to Vimeo. Keep the app open until the upload finishes.
    </p>
  </form>
</template>
