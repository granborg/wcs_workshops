<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import DatePicker from 'primevue/datepicker'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import type { CreatorSummary, LibraryTree } from '@/types'
import { api, errorMessage, type NewVideo } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { useVideoUpload } from '@/composables/useVideoUpload'
import { folderPaths, pathTo } from '@/utils/tree'
import { toDateOnly } from '@/utils/format'
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

// The collection to add to: the user's own, or one where they are an admin (?creator= from the collection page).
const collections = ref<CreatorSummary[]>([])
const creatorId = ref(typeof route.query.creator === 'string' ? route.query.creator : (auth.me?.id ?? ''))
const tree = ref<LibraryTree | null>(null)
const file = ref<File | null>(null)
const title = ref('')
// The date of the video, for example the day of the workshop. Today by default; the creator can change or clear it.
const videoDate = ref<Date | null>(new Date())
const directoryId = ref(typeof route.query.dir === 'string' ? route.query.dir : '')
const creating = ref(false)
const error = ref<string | null>(null)

// An admin sees the folders they can see, but can add only to the folders they manage.
const folders = computed(() => {
  const t = tree.value
  if (!t) return []
  const adminIds = new Set(t.adminDirectoryIds)
  const manages = (id: string) => t.canManage || pathTo(t.root, id).some((n) => adminIds.has(n.id))
  return folderPaths(t.root, t.creator.name).map((d) => ({
    id: d.id,
    label: d.path,
    disabled: !manages(d.id),
  }))
})
const busy = computed(() => uploading.value || creating.value)
const canSubmit = computed(() => !!title.value.trim() && !!directoryId.value && (mode.value === 'later' || !!file.value))

async function loadTree() {
  error.value = null
  try {
    tree.value = await api.library.tree(creatorId.value)
    if (!folders.value.some((f) => f.id === directoryId.value && !f.disabled)) {
      directoryId.value = folders.value.find((f) => !f.disabled)?.id ?? ''
    }
  } catch (err) {
    tree.value = null
    error.value = errorMessage(err)
  }
}

onMounted(async () => {
  if (!auth.me) return
  void loadTree()
  try {
    collections.value = (await api.library.list()).filter((c) => c.own || c.admin)
  } catch (err) {
    error.value = errorMessage(err)
  }
})

watch(creatorId, () => {
  directoryId.value = ''
  void loadTree()
})

// Suggest the file name as the title.
watch(file, (f) => {
  if (f && !title.value) title.value = f.name.replace(/\.[^.]+$/, '')
})

async function start() {
  if (!canSubmit.value) return
  error.value = null
  const video: NewVideo = {
    title: title.value.trim(),
    directoryId: directoryId.value,
    videoDate: videoDate.value ? toDateOnly(videoDate.value) : undefined,
  }
  try {
    if (mode.value === 'later') {
      creating.value = true
      await api.videos.placeholder(video)
    } else {
      await upload(file.value!, (size) => api.videos.ticket(video, size))
    }
    await router.push({ name: 'collection', params: { creatorId: creatorId.value }, query: { dir: directoryId.value } })
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
      <label for="video-date">Date</label>
      <DatePicker
        v-model="videoDate"
        input-id="video-date"
        :disabled="busy"
        show-icon
        icon-display="input"
        show-button-bar
        fluid
      />
      <span class="hint">For example, the day of the workshop. You can sort videos by this date.</span>
    </div>

    <div v-if="collections.length > 1" class="field">
      <label for="collection">Collection</label>
      <Select
        v-model="creatorId"
        input-id="collection"
        :options="collections"
        option-label="name"
        option-value="id"
        :disabled="busy"
        fluid
      />
      <span class="hint">You can add videos to your own collection and to the folders where you are an admin.</span>
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
