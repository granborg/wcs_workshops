<script setup lang="ts">
// Progress bar and status line for useVideoUpload.
import ProgressBar from 'primevue/progressbar'
import type { UploadPhase } from '@/composables/useVideoUpload'

defineProps<{ phase: UploadPhase; progress: number }>()
</script>

<template>
  <div class="field">
    <ProgressBar :value="progress" :mode="phase === 'uploading' ? 'determinate' : 'indeterminate'" style="height: 10px" />
    <span class="hint">
      <template v-if="phase === 'reserving'">Preparing upload…</template>
      <template v-else-if="phase === 'uploading'">Uploading {{ progress }}%</template>
      <template v-else>Finishing…</template>
    </span>
  </div>
</template>
