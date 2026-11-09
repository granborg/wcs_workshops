<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { DirectoryNode } from '@wcs/shared'
import { api, errorMessage } from '@/services/api'
import { countVideos, flatten, subtreeIds } from '@/utils/tree'
import BaseDialog from './BaseDialog.vue'

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
  return flatten(props.root)
    .filter((d) => !excluded.has(d.id))
    .map((d) => ({ ...d, label: d.depth === 0 ? props.creatorName : d.name }))
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
    <template v-if="videoCount === 0">
      <p>This folder and its sub-folders hold no videos. Anyone with access to this folder loses it.</p>
    </template>
    <template v-else>
      <p>
        This folder and its sub-folders hold {{ videoCount }} video{{ videoCount === 1 ? '' : 's' }}. What should happen
        to them?
      </p>
      <label class="radio">
        <input v-model="mode" type="radio" value="move" />
        <span>
          <strong>Move them to another folder</strong>
          <span class="muted small"><br />Access granted on individual videos stays in place.</span>
        </span>
      </label>
      <div v-if="mode === 'move'" class="field">
        <label for="delete-target">Destination folder</label>
        <select id="delete-target" v-model="target">
          <option v-for="o in options" :key="o.id" :value="o.id">
            {{ '  '.repeat(o.depth) }}{{ o.label }}
          </option>
        </select>
      </div>
      <label class="radio">
        <input v-model="mode" type="radio" value="delete" />
        <span>
          <strong style="color: var(--danger)">Delete all {{ videoCount }} videos</strong>
          <span class="muted small"><br />Removed from Vimeo as well. This cannot be undone.</span>
        </span>
      </label>
    </template>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="btn-row">
      <button type="button" class="btn" :disabled="busy" @click="emit('close')">Cancel</button>
      <button
        type="button"
        class="btn danger"
        :disabled="busy || (videoCount > 0 && mode === 'move' && !target)"
        @click="confirm"
      >
        Delete folder
      </button>
    </div>
  </BaseDialog>
</template>
