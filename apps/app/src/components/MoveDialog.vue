<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Button from 'primevue/button'
import Message from 'primevue/message'
import type { DirectoryNode } from '@/types'
import { api, errorMessage } from '@/services/api'
import { findNode, flatten, subtreeIds } from '@/utils/tree'
import BaseDialog from './BaseDialog.vue'
import FolderSelect from './FolderSelect.vue'

export interface MoveTarget {
  kind: 'directory' | 'video'
  id: string
  name: string
  /** Current parent directory (for a directory) or containing directory (for a video). */
  currentDirectoryId: string
}

const props = defineProps<{ open: boolean; target: MoveTarget | null; root: DirectoryNode; creatorName: string }>()
const emit = defineEmits<{ close: []; moved: [] }>()

const choice = ref('')
const busy = ref(false)
const error = ref<string | null>(null)

const options = computed(() => {
  if (!props.target) return []
  const excluded = new Set<string>([props.target.currentDirectoryId])
  if (props.target.kind === 'directory') {
    const node = findNode(props.root, props.target.id)
    if (node) for (const id of subtreeIds(node)) excluded.add(id)
  }
  return flatten(props.root).map((d) => ({
    id: d.id,
    depth: d.depth,
    label: d.depth === 0 ? props.creatorName : d.name,
    disabled: excluded.has(d.id),
  }))
})

watch(
  () => props.open,
  (open) => {
    if (open) {
      choice.value = options.value.find((o) => !o.disabled)?.id ?? ''
      error.value = null
    }
  },
)

async function confirm() {
  if (!props.target || !choice.value) return
  busy.value = true
  error.value = null
  try {
    if (props.target.kind === 'directory') await api.directories.move(props.target.id, choice.value)
    else await api.videos.move(props.target.id, choice.value)
    emit('moved')
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <BaseDialog :open="open" :title="`Move ${target?.name ?? ''}`" @close="emit('close')">
    <Message severity="info" size="small" class="mb">
      Moving changes who can see this item. Access granted on the item itself follows it. Access inherited from the
      old folder stops. Access inherited from the new folder starts.
    </Message>
    <div class="field">
      <label for="move-select">Destination folder</label>
      <FolderSelect v-model="choice" :options="options" input-id="move-select" />
    </div>
    <Message v-if="error" severity="error">{{ error }}</Message>
    <div class="btn-row">
      <Button label="Cancel" severity="secondary" text :disabled="busy" @click="emit('close')" />
      <Button label="Move" icon="pi pi-arrows-alt" :loading="busy" :disabled="!choice" @click="confirm" />
    </div>
  </BaseDialog>
</template>
