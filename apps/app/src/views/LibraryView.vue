<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Skeleton from 'primevue/skeleton'
import type { CreatorSummary } from '@/types'
import { api, errorMessage } from '@/services/api'
import { useDelayedFlag } from '@/composables/useDelayedFlag'

const router = useRouter()
const creators = ref<CreatorSummary[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const slow = useDelayedFlag(loading)

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
  <Message v-if="error" severity="error" class="mb">{{ error }}</Message>

  <template v-if="loading">
    <ul v-if="slow" class="list">
      <li v-for="i in 3" :key="i" class="row">
        <Skeleton shape="square" size="40px" border-radius="10px" />
        <div class="grow"><Skeleton width="60%" /></div>
      </li>
    </ul>
  </template>

  <div v-else-if="creators.length === 0" class="empty">
    <i class="pi pi-qrcode" aria-hidden="true" />
    <p><strong>Nothing here yet</strong></p>
    <p>Scan a creator's QR code to add their videos to your library.</p>
    <Button as="router-link" :to="{ name: 'scan' }" label="Scan a QR code" icon="pi pi-camera" />
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
      <span class="lead"><i class="pi" :class="c.own ? 'pi-star' : 'pi-folder'" aria-hidden="true" /></span>
      <div class="grow">
        <div class="title" :style="c.own ? 'font-weight: 700' : ''">{{ c.name }}</div>
        <div class="sub">{{ c.own ? 'Your collection' : 'Shared with you' }}</div>
      </div>
      <i class="pi pi-chevron-right trail" aria-hidden="true" />
    </li>
  </ul>
</template>
