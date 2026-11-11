<script setup lang="ts">
import Select from 'primevue/select'

export interface FolderOption {
  id: string
  /** Full path of the folder, for example "Ana / Season 1 / Week 1" (see folderPaths). */
  label: string
  disabled?: boolean
}

defineProps<{ options: FolderOption[]; inputId: string; disabled?: boolean }>()
const model = defineModel<string>({ required: true })
</script>

<template>
  <Select
    v-model="model"
    :options="options"
    option-label="label"
    option-value="id"
    option-disabled="disabled"
    :input-id="inputId"
    :disabled="disabled"
    placeholder="Select a folder"
    fluid
  >
    <template #option="{ option }">
      <span class="folder-option">
        <i class="pi pi-folder muted" aria-hidden="true" />
        <span class="folder-path">{{ option.label }}</span>
      </span>
    </template>
  </Select>
</template>

<style scoped>
.folder-option {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

/* A long path wraps in the list instead of making the overlay wider than the screen. */
.folder-path {
  white-space: normal;
  overflow-wrap: anywhere;
}
</style>
