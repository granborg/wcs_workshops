<script setup lang="ts">
import type { CreatorSummary } from '@wcs/shared'

defineProps<{ creator: CreatorSummary }>()
</script>

<template>
  <img v-if="creator.bannerUrl" :src="creator.bannerUrl" :alt="creator.name" class="banner" />
  <h2 v-else :style="creator.own ? 'font-weight: 700' : ''">{{ creator.name }}</h2>
</template>
