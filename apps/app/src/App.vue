<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import ConfirmDialog from 'primevue/confirmdialog'
import Toast from 'primevue/toast'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const auth = useAuthStore()

const showChrome = computed(() => route.name !== 'sign-in')
const title = computed(() => (route.meta.title as string | undefined) ?? 'WCS Workshops')

const tabs = computed(() => {
  const list = [
    { name: 'library', icon: 'pi-book', label: 'Library', match: ['library', 'collection', 'video'] },
    { name: 'scan', icon: 'pi-qrcode', label: 'Scan QR code', match: ['scan'] },
  ]
  if (auth.isCreator) list.push({ name: 'upload', icon: 'pi-upload', label: 'Upload', match: ['upload'] })
  list.push({ name: 'about', icon: 'pi-question-circle', label: 'How it works', match: ['about'] })
  list.push({ name: 'settings', icon: 'pi-cog', label: 'Settings', match: ['settings', 'audit'] })
  return list
})

function isActive(match: string[]) {
  return typeof route.name === 'string' && match.includes(route.name)
}
</script>

<template>
  <header v-if="showChrome" class="appbar">
    <h1>{{ title }}</h1>
    <nav aria-label="Main">
      <!-- Icons only. aria-label keeps each link named for screen readers. -->
      <RouterLink
        v-for="t in tabs"
        :key="t.name"
        :to="{ name: t.name }"
        class="navbtn"
        :class="{ active: isActive(t.match) }"
        :aria-label="t.label"
        :aria-current="isActive(t.match) ? 'page' : undefined"
      >
        <i class="pi" :class="t.icon" aria-hidden="true" />
      </RouterLink>
    </nav>
  </header>

  <main class="page">
    <RouterView />
  </main>

  <Toast position="top-center" />
  <ConfirmDialog :style="{ width: 'min(92vw, 420px)' }" :draggable="false" />
</template>
