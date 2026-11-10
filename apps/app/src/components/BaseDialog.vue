<script setup lang="ts">
import Dialog from 'primevue/dialog'

defineProps<{ open: boolean; title?: string }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <!-- PrimeVue renders the dialog under <body>; the print CSS in style.css relies on that. -->
  <Dialog
    :visible="open"
    modal
    dismissable-mask
    :draggable="false"
    :header="title"
    :style="{ width: 'min(92vw, 460px)' }"
    @update:visible="(v: boolean) => !v && emit('close')"
  >
    <slot v-if="open" />
  </Dialog>
</template>
