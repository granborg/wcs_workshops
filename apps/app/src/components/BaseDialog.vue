<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'

const props = defineProps<{ open: boolean; title?: string }>()
const emit = defineEmits<{ close: [] }>()

const el = ref<HTMLDialogElement | null>(null)

function sync() {
  const d = el.value
  if (!d) return
  if (props.open && !d.open) d.showModal()
  if (!props.open && d.open) d.close()
}

onMounted(sync)
watch(() => props.open, sync)
</script>

<template>
  <!-- Teleported to <body>: the print CSS hides every child of <body> except the dialog. -->
  <Teleport to="body">
    <dialog ref="el" @cancel.prevent="emit('close')">
      <header v-if="title" class="dialog-head">
        <h2>{{ title }}</h2>
        <button class="iconbtn no-print" type="button" aria-label="Close" @click="emit('close')">
          <Icon name="close" />
        </button>
      </header>
      <slot v-if="open" />
    </dialog>
  </Teleport>
</template>
