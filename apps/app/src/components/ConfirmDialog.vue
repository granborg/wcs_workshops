<script setup lang="ts">
import BaseDialog from './BaseDialog.vue'

defineProps<{
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
  busy?: boolean
}>()
const emit = defineEmits<{ close: []; confirm: [] }>()
</script>

<template>
  <BaseDialog :open="open" :title="title" @close="emit('close')">
    <p>{{ message }}</p>
    <div class="btn-row">
      <button type="button" class="btn" :disabled="busy" @click="emit('close')">Cancel</button>
      <button type="button" class="btn" :class="danger ? 'danger' : 'primary'" :disabled="busy" @click="emit('confirm')">
        {{ confirmLabel ?? 'Confirm' }}
      </button>
    </div>
  </BaseDialog>
</template>
