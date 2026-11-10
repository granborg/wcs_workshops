<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import ConfirmDialog from 'primevue/confirmdialog'
import Toast from 'primevue/toast'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const auth = useAuthStore()

const showChrome = computed(() => route.name !== 'sign-in')

// Labels are short so that five tabs fit across a phone screen.
const tabs = computed(() => {
  const list = [
    { name: 'library', icon: 'pi-book', label: 'Library', match: ['library', 'collection', 'video'] },
    { name: 'scan', icon: 'pi-qrcode', label: 'Scan', match: ['scan'] },
  ]
  if (auth.isCreator) list.push({ name: 'upload', icon: 'pi-upload', label: 'Upload', match: ['upload'] })
  list.push({ name: 'about', icon: 'pi-question-circle', label: 'About', match: ['about'] })
  list.push({ name: 'settings', icon: 'pi-cog', label: 'Settings', match: ['settings', 'audit'] })
  return list
})

function isActive(match: string[]) {
  return typeof route.name === 'string' && match.includes(route.name)
}
</script>

<template>
  <header v-if="showChrome" class="appbar">
    <nav aria-label="Main">
      <RouterLink
        v-for="t in tabs"
        :key="t.name"
        :to="{ name: t.name }"
        class="navbtn"
        :class="{ active: isActive(t.match) }"
        :aria-current="isActive(t.match) ? 'page' : undefined"
      >
        <i class="pi" :class="t.icon" aria-hidden="true" />
        <span class="navlabel">{{ t.label }}</span>
      </RouterLink>
    </nav>
  </header>

  <main class="page">
    <RouterView />
  </main>

  <Toast position="top-center" />
  <ConfirmDialog :style="{ width: 'min(92vw, 420px)' }" :draggable="false" />
</template>
