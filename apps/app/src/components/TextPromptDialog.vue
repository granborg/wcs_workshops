<script setup lang="ts">
import { ref, watch } from 'vue'
import BaseDialog from './BaseDialog.vue'

const props = defineProps<{
  open: boolean
  title: string
  label: string
  initial?: string
  confirmLabel?: string
}>()
const emit = defineEmits<{ close: []; submit: [value: string] }>()

const value = ref('')
const error = ref<string | null>(null)

watch(
  () => props.open,
  (open) => {
    if (open) {
      value.value = props.initial ?? ''
      error.value = null
    }
  },
)

function submit() {
  const v = value.value.trim()
  if (!v) {
    error.value = 'Enter a name'
    return
  }
  emit('submit', v)
}
</script>

<template>
  <BaseDialog :open="open" :title="title" @close="emit('close')">
    <form @submit.prevent="submit">
      <div class="field">
        <label for="prompt-input">{{ label }}</label>
        <input id="prompt-input" v-model="value" type="text" autocomplete="off" autofocus />
      </div>
      <p v-if="error" class="error">{{ error }}</p>
      <div class="btn-row">
        <button type="button" class="btn" @click="emit('close')">Cancel</button>
        <button type="submit" class="btn primary">{{ confirmLabel ?? 'Save' }}</button>
      </div>
    </form>
  </BaseDialog>
</template>
