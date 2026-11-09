<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { DirectoryNode } from '@wcs/shared'
import { api, errorMessage } from '@/services/api'
import { findNode, flatten, subtreeIds } from '@/utils/tree'
import BaseDialog from './BaseDialog.vue'

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
    ...d,
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
    <p class="muted small">
      Moving changes who can see this item. Access granted on the item itself follows it. Access inherited from
      the old folder stops. Access inherited from the new folder starts.
    </p>
    <div class="field">
      <label for="move-select">Destination folder</label>
      <select id="move-select" v-model="choice">
        <option v-for="o in options" :key="o.id" :value="o.id" :disabled="o.disabled">
          {{ '  '.repeat(o.depth) }}{{ o.label }}
        </option>
      </select>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="btn-row">
      <button type="button" class="btn" :disabled="busy" @click="emit('close')">Cancel</button>
      <button type="button" class="btn primary" :disabled="busy || !choice" @click="confirm">Move</button>
    </div>
  </BaseDialog>
</template>
