<script setup lang="ts">
// A large button that opens the device's file picker for one video file.
import { ref } from 'vue'
import { formatBytes } from '@/utils/format'

defineProps<{ disabled?: boolean; inputId?: string }>()
const file = defineModel<File | null>({ default: null })
const input = ref<HTMLInputElement | null>(null)

function onChange(e: Event) {
  file.value = (e.target as HTMLInputElement).files?.[0] ?? null
}
</script>

<template>
  <input :id="inputId" ref="input" type="file" accept="video/*" hidden :disabled="disabled" @change="onChange" />
  <button type="button" class="picker" :disabled="disabled" @click="input?.click()">
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
