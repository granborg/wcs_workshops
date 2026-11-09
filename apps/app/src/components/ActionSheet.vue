<script setup lang="ts">
import BaseDialog from './BaseDialog.vue'
import Icon from './Icon.vue'

export interface SheetAction {
  key: string
  label: string
  icon: string
  danger?: boolean
}

defineProps<{ open: boolean; title: string; actions: SheetAction[] }>()
const emit = defineEmits<{ close: []; pick: [key: string] }>()
</script>

<template>
  <BaseDialog :open="open" :title="title" @close="emit('close')">
    <ul class="list">
      <li
        v-for="a in actions"
        :key="a.key"
        class="row clickable"
        :class="{ danger: a.danger }"
        role="button"
        tabindex="0"
        @click="emit('pick', a.key)"
        @keydown.enter="emit('pick', a.key)"
      >
        <span class="lead" :style="a.danger ? 'color: var(--danger)' : ''"><Icon :name="a.icon" /></span>
        <span class="grow title" :style="a.danger ? 'color: var(--danger)' : ''">{{ a.label }}</span>
      </li>
    </ul>
  </BaseDialog>
</template>
