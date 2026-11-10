<script setup lang="ts">
import { ref, watch } from 'vue'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import BaseDialog from './BaseDialog.vue'

const props = defineProps<{
  open: boolean
  title: string
  label: string
  initial?: string
  confirmLabel?: string
  busy?: boolean
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
        <InputText id="prompt-input" v-model="value" autocomplete="off" autofocus fluid :invalid="!!error" />
      </div>
      <Message v-if="error" severity="error" size="small" variant="simple">{{ error }}</Message>
      <div class="btn-row">
        <Button label="Cancel" severity="secondary" text @click="emit('close')" />
        <Button type="submit" :label="confirmLabel ?? 'Save'" :loading="busy" />
      </div>
    </form>
  </BaseDialog>
</template>
