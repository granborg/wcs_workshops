<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { CreatorSummary } from '@/types'
import { api, errorMessage } from '@/services/api'
import Icon from '@/components/Icon.vue'

const router = useRouter()
const creators = ref<CreatorSummary[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    creators.value = await api.library.list()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

onMounted(load)

function open(c: CreatorSummary) {
  void router.push({ name: 'collection', params: { creatorId: c.id } })
}
</script>

<template>
  <p v-if="error" class="error">{{ error }}</p>
  <p v-if="loading" class="muted">Loading…</p>

  <div v-else-if="creators.length === 0" class="empty">
    <Icon name="qr" style="width: 48px; height: 48px; margin-bottom: 12px" />
    <p>Nothing here yet.</p>
    <p>Scan a creator's QR code to add their videos to your library.</p>
    <RouterLink class="btn primary" :to="{ name: 'scan' }">Scan a QR code</RouterLink>
  </div>

  <ul v-else class="list">
    <li
      v-for="c in creators"
      :key="c.id"
      class="row clickable"
      role="button"
      tabindex="0"
      @click="open(c)"
      @keydown.enter="open(c)"
    >
      <span class="lead"><Icon name="folder" /></span>
      <div class="grow">
        <div class="title" :class="{ bold: c.own }">{{ c.name }}</div>
        <div v-if="c.own" class="sub">Your collection</div>
      </div>
      <span class="lead"><Icon name="chevron-right" /></span>
    </li>
  </ul>
</template>
