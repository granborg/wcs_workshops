<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Button from 'primevue/button'
import Message from 'primevue/message'
import RadioButton from 'primevue/radiobutton'
import type { DirectoryNode } from '@/types'
import { api, errorMessage } from '@/services/api'
import { countVideos, folderPaths, subtreeIds } from '@/utils/tree'
import BaseDialog from './BaseDialog.vue'
import FolderSelect from './FolderSelect.vue'

const props = defineProps<{ open: boolean; node: DirectoryNode | null; root: DirectoryNode; creatorName: string }>()
const emit = defineEmits<{ close: []; deleted: [] }>()

const mode = ref<'delete' | 'move'>('move')
const target = ref('')
const busy = ref(false)
const error = ref<string | null>(null)

const videoCount = computed(() => (props.node ? countVideos(props.node) : 0))

const options = computed(() => {
  if (!props.node) return []
  const excluded = new Set(subtreeIds(props.node))
  return folderPaths(props.root, props.creatorName)
    .filter((d) => !excluded.has(d.id))
    .map((d) => ({ id: d.id, label: d.path }))
})

watch(
  () => props.open,
  (open) => {
    if (open) {
      mode.value = 'move'
      target.value = props.node?.parentId ?? options.value[0]?.id ?? ''
      error.value = null
    }
  },
)

async function confirm() {
  if (!props.node) return
  busy.value = true
  error.value = null
  try {
    if (videoCount.value === 0 || mode.value === 'delete') {
      await api.directories.remove(props.node.id, 'delete')
    } else {
      await api.directories.remove(props.node.id, 'move', target.value)
    }
    emit('deleted')
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <BaseDialog :open="open" :title="`Delete ${node?.name ?? 'folder'}`" @close="emit('close')">
    <p v-if="videoCount === 0">This folder and its sub-folders hold no videos. Anyone with access to this folder loses it.</p>
    <template v-else>
      <p>
        This folder and its sub-folders hold {{ videoCount }} video{{ videoCount === 1 ? '' : 's' }}. What should happen
        to them?
      </p>
      <div class="stack">
        <label class="choice" for="del-move">
          <RadioButton v-model="mode" input-id="del-move" name="del-mode" value="move" />
          <span>
            <strong>Move them to another folder</strong>
            <span class="hint" style="display: block">Access granted on individual videos stays in place.</span>
          </span>
        </label>
        <div v-if="mode === 'move'" class="field" style="margin-left: 32px">
          <label for="delete-target">Destination folder</label>
          <FolderSelect v-model="target" :options="options" input-id="delete-target" />
        </div>
        <label class="choice" for="del-delete">
          <RadioButton v-model="mode" input-id="del-delete" name="del-mode" value="delete" />
          <span>
            <strong style="color: var(--p-red-500)">Delete all {{ videoCount }} videos</strong>
            <span class="hint" style="display: block">Removed from Vimeo as well. This cannot be undone.</span>
          </span>
        </label>
      </div>
    </template>
    <Message v-if="error" severity="error" style="margin-top: 12px">{{ error }}</Message>
    <div class="btn-row">
      <Button label="Cancel" severity="secondary" text :disabled="busy" @click="emit('close')" />
      <Button
        label="Delete folder"
        icon="pi pi-trash"
        severity="danger"
        :loading="busy"
        :disabled="videoCount > 0 && mode === 'move' && !target"
        @click="confirm"
      />
    </div>
  </BaseDialog>
</template>

<style scoped>
.choice {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  cursor: pointer;
}

.choice :deep(.p-radiobutton) {
  margin-top: 2px;
}
</style>
