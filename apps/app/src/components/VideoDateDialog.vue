<script setup lang="ts">
// Sets or clears the date of a video.
import { ref, watch } from 'vue'
import Button from 'primevue/button'
import DatePicker from 'primevue/datepicker'
import BaseDialog from './BaseDialog.vue'
import { fromDateOnly, toDateOnly } from '@/utils/format'

const props = defineProps<{
  open: boolean
  name: string
  /** The current date as YYYY-MM-DD, or null. */
  initial: string | null
  busy?: boolean
}>()
const emit = defineEmits<{ close: []; submit: [value: string | null] }>()

const value = ref<Date | null>(null)

watch(
  () => props.open,
  (open) => {
    if (open) value.value = props.initial ? fromDateOnly(props.initial) : null
  },
)

function submit() {
  emit('submit', value.value ? toDateOnly(value.value) : null)
}
</script>

<template>
  <BaseDialog :open="open" :title="`Date of ${name}`" @close="emit('close')">
    <form @submit.prevent="submit">
      <div class="field">
        <label for="video-date-input">Date</label>
        <DatePicker v-model="value" input-id="video-date-input" show-icon icon-display="input" show-button-bar fluid />
        <span class="hint">Leave empty for no date. Videos with no date show last when sorted by date.</span>
      </div>
      <div class="btn-row">
        <Button label="Cancel" severity="secondary" text @click="emit('close')" />
        <Button type="submit" label="Save" :loading="busy" />
      </div>
    </form>
  </BaseDialog>
</template>
