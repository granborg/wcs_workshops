<script setup lang="ts">
import Select from 'primevue/select'

export interface FolderOption {
  id: string
  label: string
  depth: number
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
      <span :style="{ paddingLeft: `${option.depth * 16}px`, display: 'inline-flex', alignItems: 'center', gap: '8px' }">
        <i class="pi pi-folder muted" aria-hidden="true" />
        {{ option.label }}
      </span>
    </template>
  </Select>
</template>
